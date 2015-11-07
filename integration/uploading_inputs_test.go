package integration_test

import (
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"

	"github.com/concourse/atc"
)

var _ = Describe("Fly CLI", func() {
	var tmpdir string
	var buildDir string
	var taskConfigPath string

	var atcServer *ghttp.Server
	var hangUntilConnectionDies <-chan struct{}

	BeforeEach(func() {
		var err error
		tmpdir, err = ioutil.TempDir("", "fly-build-dir")
		Expect(err).NotTo(HaveOccurred())

		buildDir = filepath.Join(tmpdir, "fixture")

		err = os.Mkdir(buildDir, 0755)
		Expect(err).NotTo(HaveOccurred())

		taskConfigPath = filepath.Join(buildDir, "task.yml")

		err = ioutil.WriteFile(
			taskConfigPath,
			[]byte(`---
platform: some-platform

image: ubuntu

inputs:
- name: fixture

params:
  FOO: bar
  BAZ: buzz
  X: 1

run:
  path: find
  args: [.]
`),
			0644,
		)
		Expect(err).NotTo(HaveOccurred())

		atcServer = ghttp.NewServer()

		hangUntilConnectionDies = make(chan struct{})

		atcServer.AppendHandlers(
			ghttp.CombineHandlers(
				ghttp.VerifyRequest("POST", "/api/v1/pipes"),
				ghttp.RespondWithJSONEncoded(http.StatusCreated, atc.Pipe{
					ID: "some-pipe-id",
				}),
			),
		)
		atcServer.RouteToHandler("POST", "/api/v1/builds",
			ghttp.CombineHandlers(
				ghttp.VerifyRequest("POST", "/api/v1/builds"),
				func(w http.ResponseWriter, r *http.Request) {
					http.SetCookie(w, &http.Cookie{
						Name:    "Some-Cookie",
						Value:   "some-cookie-data",
						Path:    "/",
						Expires: time.Now().Add(1 * time.Minute),
					})
				},
				ghttp.RespondWith(201, `{"id":128}`),
			),
		)
		atcServer.RouteToHandler("PUT", "/api/v1/pipes/some-pipe-id",
			ghttp.CombineHandlers(
				ghttp.VerifyRequest("PUT", "/api/v1/pipes/some-pipe-id"),
				func(w http.ResponseWriter, req *http.Request) {
					atcServer.CloseClientConnections()
					//close(crashAtc)
				},
			),
		)
	})

	AfterEach(func() {
		err := os.RemoveAll(tmpdir)
		Expect(err).NotTo(HaveOccurred())
	})

	Context("when the atc goes offline before uploading inputs", func() {
		BeforeEach(func() {
			atcServer.RouteToHandler("GET", "/api/v1/builds/128/events",
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/api/v1/builds/128/events"),
					ghttp.RespondWith(200, ""),
				),
			)
		})

		FIt("exits 1", func() {
			flyCmd := exec.Command(flyPath, "-t", atcServer.URL(), "e", "-c", taskConfigPath)
			flyCmd.Dir = buildDir

			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())

			<-sess.Exited
			Expect(sess.ExitCode()).To(Equal(1))

			Expect(sess.Err).Should(gbytes.Say("upload request failed"))
			Expect(sess.Err).Should(gbytes.Say("failed to attach to stream"))
		})
	})

	Context("when the atc goes offline while uploading inputs", func() {
		BeforeEach(func() {
			atcServer.RouteToHandler("GET", "/api/v1/builds/128/events",
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/api/v1/builds/128/events"),
					func(w http.ResponseWriter, req *http.Request) {
						<-hangUntilConnectionDies
					},
				),
			)
		})

		FIt("exits 1", func() {
			flyCmd := exec.Command(flyPath, "-t", atcServer.URL(), "e", "-c", taskConfigPath)
			flyCmd.Dir = buildDir

			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())

			<-sess.Exited
			Expect(sess.ExitCode()).To(Equal(1))

			Expect(sess.Err).Should(gbytes.Say("upload request failed"))
			Expect(sess.Err).Should(gbytes.Say("failed to attach to stream"))
		})
	})
})
