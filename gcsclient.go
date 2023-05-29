package gcsresource

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"cloud.google.com/go/storage"
	"github.com/cheggaaa/pb/v3"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

//go:generate counterfeiter -o fakes/fake_gcsclient.go . GCSClient
type GCSClient interface {
	BucketObjects(bucketName, prefix string) ([]string, error)
	ObjectGenerations(bucketName, objectPath string) ([]int64, error)
	DownloadFile(bucketName, objectPath string, generation int64, localPath string) error
	UploadFile(bucketName, objectPath, objectContentType, localPath, predefinedACL, cacheControl string) (int64, error)
	URL(bucketName, objectPath string, generation int64) (string, error)
	DeleteObject(bucketName, objectPath string, generation int64) error
	GetBucketObjectInfo(bucketName, objectPath string) (*storage.ObjectAttrs, error)
}

type gcsclient struct {
	storageService *storage.Client
	progressOutput io.Writer
}

func NewGCSClient(
	progressOutput io.Writer,
	jsonKey string,
) (GCSClient, error) {
	var err error
	userAgent := "gcs-resource/0.0.1"

	var storageService *storage.Client
	if jsonKey == "" {
		ctx := context.Background()
		storageService, err = storage.NewClient(ctx, option.WithUserAgent(userAgent))
		if err != nil {
			return &gcsclient{}, err
		}
	} else {
		ctx := context.Background()
		storageService, err = storage.NewClient(ctx, option.WithUserAgent(userAgent), option.WithCredentialsJSON([]byte(jsonKey)))
		if err != nil {
			return &gcsclient{}, err
		}
	}

	return &gcsclient{
		storageService: storageService,
		progressOutput: progressOutput,
	}, nil
}

func (g *gcsclient) BucketObjects(bucketName, prefix string) ([]string, error) {
	bucketObjects, err := g.getBucketObjects(bucketName, prefix)
	if err != nil {
		return []string{}, err
	}

	return bucketObjects, nil
}

func (g *gcsclient) ObjectGenerations(bucketName, objectPath string) ([]int64, error) {
	isBucketVersioned, err := g.getBucketVersioning(bucketName)
	if err != nil {
		return []int64{}, err
	}

	if !isBucketVersioned {
		return []int64{}, errors.New("bucket is not versioned")
	}

	objectGenerations, err := g.getObjectGenerations(bucketName, objectPath)
	if err != nil {
		return []int64{}, err
	}

	return objectGenerations, nil
}

func (g *gcsclient) DownloadFile(bucketName, objectPath string, generation int64, localPath string) error {
	isBucketVersioned, err := g.getBucketVersioning(bucketName)
	if err != nil {
		return err
	}

	if !isBucketVersioned && generation != 0 {
		return errors.New("bucket is not versioned")
	}

	attrs, err := g.GetBucketObjectInfo(bucketName, objectPath)
	if err != nil {
		return err
	}

	progress := g.newProgressBar(int64(attrs.Size))
	defer progress.Finish()
	progress.Start()

	localFile, err := os.Create(localPath)
	if err != nil {
		return err
	}
	defer localFile.Close()
	ctx := context.Background()
	objectHandle := g.storageService.Bucket(bucketName).Object(objectPath)
	if generation != 0 {
		objectHandle = objectHandle.Generation(generation)
	}
	rc, err := objectHandle.NewReader(ctx)
	if err != nil {
		return err
	}
	defer rc.Close()

	_, err = io.Copy(localFile, progress.NewProxyReader(rc))
	if err != nil {
		return err
	}

	return nil
}

func (g *gcsclient) UploadFile(bucketName, objectPath, objectContentType, localPath, predefinedACL, cacheControl string) (int64, error) {
	isBucketVersioned, err := g.getBucketVersioning(bucketName)
	if err != nil {
		return 0, err
	}

	stat, err := os.Stat(localPath)
	if err != nil {
		return 0, err
	}

	localFile, err := os.Open(localPath)
	if err != nil {
		return 0, err
	}
	defer localFile.Close()

	progress := g.newProgressBar(stat.Size())
	progress.Start()
	defer progress.Finish()

	ctx := context.Background()
	wc := g.storageService.Bucket(bucketName).Object(objectPath).NewWriter(ctx)
	if _, err = io.Copy(wc, progress.NewProxyReader(localFile)); err != nil {
		return 0, err
	}

	if err := wc.Close(); err != nil {
		return 0, err
	}

	if predefinedACL != "" || cacheControl != "" || objectContentType != "" {
		var cacheControlOption interface{}
		var contentTypeOption interface{}

		if cacheControl != "" {
			cacheControlOption = cacheControl
		}

		if objectContentType != "" {
			contentTypeOption = objectContentType
		}

		attrs := storage.ObjectAttrsToUpdate{
			ContentType:   contentTypeOption,
			CacheControl:  cacheControlOption,
			PredefinedACL: predefinedACL,
		}
		ctx = context.Background()
		_, err = g.storageService.Bucket(bucketName).Object(objectPath).Update(ctx, attrs)
		if err != nil {
			return 0, nil
		}
	}

	if isBucketVersioned {
		attrs, err := g.GetBucketObjectInfo(bucketName, objectPath)
		if err != nil {
			return 0, err
		}
		return attrs.Generation, nil
	}
	return 0, nil
}

func (g *gcsclient) URL(bucketName, objectPath string, generation int64) (string, error) {
	ctx := context.Background()
	objectHandle := g.storageService.Bucket(bucketName).Object(objectPath)
	if generation != 0 {
		objectHandle = objectHandle.Generation(generation)
	}
	attrs, err := objectHandle.Attrs(ctx)
	if err != nil {
		return "", err
	}

	var url string
	if generation != 0 {
		url = fmt.Sprintf("gs://%s/%s#%d", bucketName, objectPath, attrs.Generation)
	} else {
		url = fmt.Sprintf("gs://%s/%s", bucketName, objectPath)
	}

	return url, nil
}

func (g *gcsclient) DeleteObject(bucketName, objectPath string, generation int64) error {
	var err error
	ctx := context.Background()
	if generation != 0 {
		err = g.storageService.Bucket(bucketName).Object(objectPath).Generation(generation).Delete(ctx)
	} else {
		err = g.storageService.Bucket(bucketName).Object(objectPath).Delete(ctx)
	}
	if err != nil {
		return err
	}
	return nil
}

func (g *gcsclient) GetBucketObjectInfo(bucketName, objectPath string) (*storage.ObjectAttrs, error) {
	ctx := context.Background()
	attrs, err := g.storageService.Bucket(bucketName).Object(objectPath).Attrs(ctx)
	if err != nil {
		return nil, err
	}
	return attrs, nil
}

func (g *gcsclient) getBucketObjects(bucketName, prefix string) ([]string, error) {
	var bucketObjects []string
	ctx := context.Background()
	pageToken := ""
	query := &storage.Query{
		Delimiter: pageToken,
		Prefix:    prefix,
		Versions:  false,
	}
	objectIterator := g.storageService.Bucket(bucketName).Objects(ctx, query)
	for {
		object, err := objectIterator.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		bucketObjects = append(bucketObjects, object.Name)
	}

	return bucketObjects, nil
}

func (g *gcsclient) getBucketVersioning(bucketName string) (bool, error) {
	ctx := context.Background()
	bucket, err := g.storageService.Bucket(bucketName).Attrs(ctx)
	if err != nil {
		return false, err
	}

	return bucket.VersioningEnabled, nil
}

func (g *gcsclient) getObjectGenerations(bucketName, objectPath string) ([]int64, error) {
	var objectGenerations []int64
	ctx := context.Background()
	pageToken := ""
	query := &storage.Query{
		Delimiter: pageToken,
		Prefix:    objectPath,
		Versions:  true,
	}
	objectIterator := g.storageService.Bucket(bucketName).Objects(ctx, query)
	for {
		object, err := objectIterator.Next()

		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		if object.Name == objectPath {
			objectGenerations = append(objectGenerations, object.Generation)
		}
		objectIterator.PageInfo()
	}

	return objectGenerations, nil
}

func (g *gcsclient) newProgressBar(total int64) *pb.ProgressBar {
	const tmpl = `{{with string . "prefix"}}{{.}} {{end}}{{counters . }} {{bar . }} {{percent . }} {{speed . "%s"}} {{rtime . "ETA %s"}}{{with string . "suffix"}} {{.}}{{end}}`
	return pb.New64(total).
		SetWidth(80).
		Set(pb.Bytes, true).
		SetWriter(g.progressOutput).
		Set(pb.ReturnSymbol, "\r").
		SetRefreshRate(time.Second).
		SetTemplate(tmpl)
}
