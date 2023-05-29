package in_test

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/onsi/gomega/gexec"
)

var inPath string

var _ = BeforeSuite(func() {
	var err error

	inPath, err = gexec.Build("github.com/frodenas/gcs-resource/cmd/in")
	Expect(err).ToNot(HaveOccurred())
})

var _ = AfterSuite(func() {
	gexec.CleanupBuildArtifacts()
})

func TestIn(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "In Suite")
}

type gcsDownloadTask func(bucketName, objectPath string, generation int64, localPath string) error

func gcsDownloadTaskStub(name string) gcsDownloadTask {
	return func(bucketName, objectPath string, generation int64, localPath string) error {
		sourcePath := filepath.Join("fixtures", name)
		Expect(sourcePath).To(BeAnExistingFile())

		from, err := os.Open(sourcePath)
		Expect(err).NotTo(HaveOccurred())
		defer from.Close()

		destinationDir := filepath.Dir(localPath)

		to, err := os.OpenFile(filepath.Join(destinationDir, name), os.O_RDWR|os.O_CREATE, 0o600)
		Expect(err).NotTo(HaveOccurred())
		defer to.Close()

		_, err = io.Copy(to, from)
		Expect(err).NotTo(HaveOccurred())

		return nil
	}
}
