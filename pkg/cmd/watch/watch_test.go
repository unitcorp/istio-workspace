package watch_test

import (
	"path"
	"strings"
	"time"

	"github.com/maistra/istio-workspace/pkg/cmd/watch"

	"github.com/maistra/istio-workspace/test/shell"

	. "github.com/maistra/istio-workspace/pkg/cmd"
	. "github.com/maistra/istio-workspace/test"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/spf13/cobra"
)

var _ = Describe("Usage of ike watch command", func() {

	var watchCmd *cobra.Command

	BeforeEach(func() {
		watchCmd = watch.NewCmd()
		watchCmd.SilenceUsage = true
		watchCmd.SilenceErrors = true
		NewCmd().AddCommand(watchCmd)
	})

	Context("watching file changes", func() {

		tmpPath := NewTmpPath()
		BeforeEach(func() {
			tmpPath.SetPath(path.Dir(shell.MvnBin), path.Dir(shell.TpSleepBin), path.Dir(shell.JavaBin))
		})
		AfterEach(tmpPath.Restore)

		It("should re-build and re-run java process", func() {
			// given
			tmpDir := TmpDir(GinkgoT(), "re-run")
			code := TmpFile(GinkgoT(), tmpDir+"/watch-test/rating.java", "content")
			telepresenceLog := TmpFile(GinkgoT(), tmpDir+"/watch-test/telepresence.log", "content")
			outputChan := make(chan string)

			go shell.ExecuteCommand(outputChan, func() (string, error) {
				return Run(watchCmd).Passing(
					"--run", "java -jar rating.jar",
					"--build", "mvn clean install",
					"--dir", tmpDir+"/watch-test",
					// for testing purposes we handle file change events more frequently to avoid excessively long tests
					"--interval", "10",
				)
			})()

			// when
			time.Sleep(25 * time.Millisecond) // as tp process sleeps for 50ms, we wait before we start modifying the file

			_, _ = telepresenceLog.WriteString("modified!")
			_, _ = code.WriteString("modified!")

			// then
			var output string
			Eventually(outputChan).Should(Receive(&output))
			Expect(output).To(ContainSubstring("rating.java changed. Restarting process."))
			Expect(strings.Count(output, "mvn clean install")).To(Equal(2))
			Expect(strings.Count(output, "java -jar rating.jar")).To(Equal(2))
		})

		It("should start java process once if only log file is changing", func() {
			// given
			tmpDir := TmpDir(GinkgoT(), "start-java")
			logFile := TmpFile(GinkgoT(), tmpDir+"/watch-test/tomcat.log", "content")
			outputChan := make(chan string)

			go shell.ExecuteCommand(outputChan, func() (string, error) {
				return Run(watchCmd).Passing(
					"--run", "java -jar rating.jar",
					"--dir", tmpDir+"/watch-test",
					// for testing purposes we handle file change events more frequently to avoid excessively long tests
					"interval", "10",
				)
			})()

			// when
			time.Sleep(25 * time.Millisecond)

			_, _ = logFile.WriteString("\n>>> Server started")

			// then
			var output string
			Eventually(outputChan).Should(Receive(&output))
			Expect(output).ToNot(ContainSubstring("rating.java changed. Restarting process."))
			Expect(strings.Count(output, "java -jar rating.jar")).To(Equal(1))
		})

		It("should build and run java process only initially when changing file is excluded", func() {
			// given
			tmpDir := TmpDir(GinkgoT(), "build-run-java-excluded")
			code := TmpFile(GinkgoT(), tmpDir+"/watch-test/rating.java", "content")
			outputChan := make(chan string)
			go shell.ExecuteCommand(outputChan, func() (string, error) {
				return Run(watchCmd).Passing(
					"--run", "java -jar rating.jar",
					"--build", "mvn clean install",
					"--dir", tmpDir+"/watch-test",
					"--exclude", "*.java",
					// for testing purposes we handle file change events more frequently to avoid excessively long tests
					"--interval", "10",
				)
			})()

			// when
			time.Sleep(25 * time.Millisecond) // as tp process sleeps for 50ms, we wait before we start modifying the file

			_, _ = code.WriteString("modified!")

			// then
			var output string
			Eventually(outputChan).Should(Receive(&output))
			Expect(output).ToNot(ContainSubstring("rating.java changed. Restarting process."))
			Expect(strings.Count(output, "mvn clean install")).To(Equal(1))
			Expect(strings.Count(output, "java -jar rating.jar")).To(Equal(1))
		})

		It("should ignore build if not defined and just re-run java process on file change", func() {
			tmpDir := TmpDir(GinkgoT(), "ignore-build")
			code := TmpFile(GinkgoT(), tmpDir+"/watch-test/rating.java", "content")

			outputChan := make(chan string)
			go shell.ExecuteCommand(outputChan, func() (string, error) {
				return Run(watchCmd).Passing(
					"--run", "java -jar rating.jar",
					"--dir", tmpDir+"/watch-test",
					// for testing purposes we handle file change events more frequently to avoid excessively long tests
					"--interval", "10",
				)
			})()

			time.Sleep(25 * time.Millisecond) // as tp process sleeps for 50ms, we wait before we start modifying the file
			_, _ = code.WriteString("modified!")

			var output string
			Eventually(outputChan).Should(Receive(&output))
			Expect(output).To(ContainSubstring("rating.java changed. Restarting process."))
			Expect(strings.Count(output, "mvn clean install")).To(Equal(0))
			Expect(strings.Count(output, "java -jar rating.jar")).To(Equal(2))
		})

		It("should only re-run java process when --no-build flag specified but build defined in config", func() {
			configFile := TmpFile(GinkgoT(), "config.yaml", `watch:
  run: "java -jar config.jar"
  build: "mvn clean install"
`)
			tmpDir := TmpDir(GinkgoT(), "re-run-no-build")
			code := TmpFile(GinkgoT(), tmpDir+"/watch-test/rating.java", "content")

			outputChan := make(chan string)
			go shell.ExecuteCommand(outputChan, func() (string, error) {
				return Run(watchCmd).Passing(
					"--config", configFile.Name(),
					"--no-build",
					"--dir", tmpDir+"/watch-test",
					// for testing purposes we handle file change events more frequently to avoid excessively long tests
					"--interval", "10",
				)
			})()

			time.Sleep(25 * time.Millisecond) // as tp process sleeps for 50ms, we wait before we start modifying the file
			_, _ = code.WriteString("modified!")

			var output string
			Eventually(outputChan).Should(Receive(&output))
			Expect(output).To(ContainSubstring("rating.java changed. Restarting process."))
			Expect(strings.Count(output, "mvn clean install")).To(Equal(0))
		})
	})

})
