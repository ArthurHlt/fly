// +build !windows

package commands

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/codegangsta/cli"
	"github.com/concourse/atc"
	"github.com/kr/pty"
	"github.com/mgutz/ansi"
	"github.com/pkg/term"
	"github.com/tedsuo/rata"
)

func remoteCommand(argv []string) (string, []string) {
	var path string
	var args []string

	switch len(argv) {
	case 0:
		path = "bash"
	case 1:
		path = argv[0]
	default:
		path = argv[0]
		args = argv[1:]
	}

	return path, args
}

type containerLocator interface {
	locate(containerFingerprint) url.Values
}

type stepContainerLocator struct {
	client       *http.Client
	reqGenerator *rata.RequestGenerator
}

func (locator stepContainerLocator) locate(fingerprint containerFingerprint) url.Values {
	build := getBuild(
		locator.client,
		locator.reqGenerator,
		fingerprint.jobName,
		fingerprint.buildName,
		fingerprint.pipelineName,
	)

	reqValues := url.Values{}
	reqValues["build-id"] = []string{strconv.Itoa(build.ID)}
	reqValues["name"] = []string{fingerprint.stepName}

	if fingerprint.stepType != "" {
		reqValues["type"] = []string{fingerprint.stepType}
	}

	return reqValues
}

type checkContainerLocator struct{}

func (locator checkContainerLocator) locate(fingerprint containerFingerprint) url.Values {
	reqValues := url.Values{}

	reqValues["type"] = []string{"check"}
	reqValues["name"] = []string{fingerprint.checkName}

	if fingerprint.pipelineName != "" {
		reqValues["pipeline"] = []string{fingerprint.pipelineName}
	}

	return reqValues
}

type containerFingerprint struct {
	pipelineName string
	jobName      string
	buildName    string

	stepName string
	stepType string

	checkName string
}

func locateContainer(client *http.Client, reqGenerator *rata.RequestGenerator, fingerprint containerFingerprint) url.Values {
	var locator containerLocator

	if fingerprint.checkName == "" {
		locator = stepContainerLocator{
			client:       client,
			reqGenerator: reqGenerator,
		}
	} else {
		locator = checkContainerLocator{}
	}

	return locator.locate(fingerprint)
}

func constructRequest(reqGenerator *rata.RequestGenerator, spec atc.HijackProcessSpec, reqValues url.Values) *http.Request {
	payload, err := json.Marshal(spec)
	if err != nil {
		log.Fatalln("failed to marshal process spec:", err)
	}

	hijackReq, err := reqGenerator.CreateRequest(
		atc.Hijack,
		rata.Params{},
		bytes.NewBuffer(payload),
	)
	if err != nil {
		log.Fatalln("failed to create hijack request:", err)
	}

	if hijackReq.URL.User != nil {
		hijackReq.Header.Add("Authorization", basicAuth(hijackReq.URL.User))
		hijackReq.URL.User = nil
	}

	hijackReq.URL.RawQuery = reqValues.Encode()

	return hijackReq
}

func Hijack(c *cli.Context) {
	target := returnTarget(c.GlobalString("target"))
	insecure := c.GlobalBool("insecure")

	stepType := c.String("step-type")
	stepName := c.String("step-name")
	check := c.String("check")
	pipelineName := c.String("pipeline")
	jobName := c.String("job")
	buildName := c.String("build")

	path, args := remoteCommand(c.Args())
	privileged := true

	fingerprint := containerFingerprint{
		pipelineName: pipelineName,
		jobName:      jobName,
		buildName:    buildName,
		stepName:     stepName,
		stepType:     stepType,
		checkName:    check,
	}

	reqGenerator := rata.NewRequestGenerator(target, atc.Routes)
	tlsConfig := &tls.Config{InsecureSkipVerify: insecure}
	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
	}
	client := &http.Client{Transport: transport}

	reqValues := locateContainer(client, reqGenerator, fingerprint)

	var ttySpec *atc.HijackTTYSpec
	rows, cols, err := pty.Getsize(os.Stdin)
	if err == nil {
		ttySpec = &atc.HijackTTYSpec{
			WindowSize: atc.HijackWindowSize{
				Columns: cols,
				Rows:    rows,
			},
		}
	}

	spec := atc.HijackProcessSpec{
		Path: path,
		Args: args,
		Env:  []string{"TERM=" + os.Getenv("TERM")},
		User: "root",

		Privileged: privileged,
		TTY:        ttySpec,
	}

	hijackReq := constructRequest(reqGenerator, spec, reqValues)
	hijackResult := performHijack(hijackReq, tlsConfig)
	os.Exit(hijackResult)
}

func performHijack(hijackReq *http.Request, tlsConfig *tls.Config) int {
	conn, err := dialEndpoint(hijackReq.URL, tlsConfig)
	if err != nil {
		log.Fatalln("failed to dial hijack endpoint:", err)
	}

	clientConn := httputil.NewClientConn(conn, nil)

	resp, err := clientConn.Do(hijackReq)
	if err != nil {
		log.Fatalln("failed to hijack:", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errorContext string = "bad response when hijacking"
		var errorMessage string

		if resp.StatusCode == http.StatusNotFound {
			errorMessage = "no containers matched your search parameters"
		} else if resp.StatusCode == http.StatusMultipleChoices {
			errorContext = "more than one matching container was found"
			err = json.NewDecoder(resp.Body).Decode(&errorMessage)
			if err != nil {
				log.Fatalln("failed to decode message:", err)
			}
		} else {
			var errorMessageBuf bytes.Buffer
			resp.Write(&errorMessageBuf)
			errorMessage = errorMessageBuf.String()
		}

		log.Println(errorContext + ":\n" + errorMessage)
		resp.Body.Close()
		os.Exit(1)
	}

	return hijack(clientConn.Hijack())
}

func hijack(conn net.Conn, br *bufio.Reader) int {
	var in io.Reader

	term, err := term.Open(os.Stdin.Name())
	if err == nil {
		err = term.SetRaw()
		if err != nil {
			log.Fatalln("failed to set raw:", term)
		}

		defer term.Restore()

		in = term
	} else {
		in = os.Stdin
	}

	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(br)

	resized := make(chan os.Signal, 10)
	signal.Notify(resized, syscall.SIGWINCH)

	go func() {
		for {
			<-resized
			// TODO json race
			sendSize(encoder)
		}
	}()

	go io.Copy(&stdinWriter{encoder}, in)

	var exitStatus int
	for {
		var output atc.HijackOutput
		err := decoder.Decode(&output)
		if err != nil {
			break
		}

		if output.ExitStatus != nil {
			exitStatus = *output.ExitStatus
		} else if len(output.Error) > 0 {
			fmt.Fprintf(os.Stderr, "%s\n", ansi.Color(output.Error, "red+b"))
			exitStatus = 255
		} else if len(output.Stdout) > 0 {
			os.Stdout.Write(output.Stdout)
		} else if len(output.Stderr) > 0 {
			os.Stderr.Write(output.Stderr)
		}
	}

	return exitStatus
}

func sendSize(enc *json.Encoder) {
	rows, cols, err := pty.Getsize(os.Stdin)
	if err == nil {
		enc.Encode(atc.HijackInput{
			TTYSpec: &atc.HijackTTYSpec{
				WindowSize: atc.HijackWindowSize{
					Columns: cols,
					Rows:    rows,
				},
			},
		})
	}
}

type stdinWriter struct {
	enc *json.Encoder
}

func (w *stdinWriter) Write(d []byte) (int, error) {
	err := w.enc.Encode(atc.HijackInput{
		Stdin: d,
	})
	if err != nil {
		return 0, err
	}

	return len(d), nil
}

func basicAuth(user *url.Userinfo) string {
	username := user.Username()
	password, _ := user.Password()
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(username+":"+password))
}

var canonicalPortMap = map[string]string{
	"http":  "80",
	"https": "443",
}

func dialEndpoint(url *url.URL, tlsConfig *tls.Config) (net.Conn, error) {
	addr := canonicalAddr(url)

	if url.Scheme == "https" {
		return tls.Dial("tcp", addr, tlsConfig)
	} else {
		return net.Dial("tcp", addr)
	}
}

func canonicalAddr(url *url.URL) string {
	host, port, err := net.SplitHostPort(url.Host)
	if err != nil {
		if strings.Contains(err.Error(), "missing port in address") {
			host = url.Host
			port = canonicalPortMap[url.Scheme]
		} else {
			log.Fatalln("invalid host:", err)
		}
	}

	return net.JoinHostPort(host, port)
}
