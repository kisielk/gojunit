package main

import (
	"bufio"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"
)

type TestSuite struct {
	Name      string
	TestCases []TestCase
	Duration  time.Duration
}

type TestCase struct {
	Name     string
	Duration time.Duration
	Status   Status
	Output   bytes.Buffer
}

type Status int

const (
	Success Status = iota
	Failure
	Error
	Skipped
)

// ParseOutput parses the output of the Go test runner and returns a slice of 
// TestSuites.
func ParseOutput(r io.Reader) ([]TestSuite, error) {
	buf := bufio.NewReader(r)
	var suites []TestSuite
	var suite = new(TestSuite)
	var tc = new(TestCase)
	var readErr error
	var line string

	for ; readErr == nil; line, readErr = buf.ReadString('\n') {
		line = strings.TrimRight(line, "\n")
		switch {
		case line == "PASS" || line == "FAIL":
			continue
		case strings.HasPrefix(line, "=== RUN"):
			suite.TestCases = append(suite.TestCases, TestCase{})
			tc = &suite.TestCases[len(suite.TestCases)-1]
			fields := strings.Fields(line)
			if len(fields) > 2 {
				tc.Name = fields[2]
			}
		case strings.HasPrefix(line, "--- FAIL:"):
			fields := strings.Fields(line)
			if len(fields) > 3 {
				// trim off leading (, ignore the error
				tc.Duration, _ = time.ParseDuration(fields[3][1:] + "s")
			}
			tc.Status = Failure
		case strings.HasPrefix(line, "--- PASS:"):
			fields := strings.Fields(line)
			if len(fields) > 3 {
				// trim off leading (, ignore the error
				tc.Duration, _ = time.ParseDuration(fields[3][1:] + "s")
			}
			tc.Status = Success
		case strings.HasPrefix(line, "FAIL"):
			fields := strings.Fields(line)
			if len(fields) > 1 {
				suite.Name = fields[1]
			}
			if len(fields) > 2 {
				suite.Duration, _ = time.ParseDuration(fields[2])
			}
			suites = append(suites, *suite)
			suite = new(TestSuite)
		case strings.HasPrefix(line, "ok"):
			fields := strings.Fields(line)
			if len(fields) > 1 {
				suite.Name = fields[1]
			}
			if len(fields) > 2 {
				suite.Duration, _ = time.ParseDuration(fields[2])
			}
			suites = append(suites, *suite)
			suite = new(TestSuite)
		default:
			fmt.Fprintln(&tc.Output, line)
		}
	}
	if readErr != nil && readErr != io.EOF {
		return nil, readErr
	}
	return suites, nil
}

// XML format based on https://svn.jenkins-ci.org/trunk/hudson/dtkit/dtkit-format/dtkit-junit-model/src/main/resources/com/thalesgroup/dtkit/junit/model/xsd/junit-4.xsd

// <testsuites> XML element
type TestSuitesXML struct {
	XMLName    xml.Name `xml:"testsuites"`
	TestSuites []TestSuiteXML
}

// <testsuite> XML element
type TestSuiteXML struct {
	XMLName   xml.Name `xml:"testsuite"`
	Name      string   `xml:"name,attr"`
	Errors    int      `xml:"errors,attr"`
	Failures  int      `xml:"failures,attr"`
	Skipped   int      `xml:"skipped,attr"`
	Tests     int      `xml:"tests,attr"`
	Time      float64  `xml:"time,attr"`
	TestCases []TestCaseXML
}

// <testcase> XML element
type TestCaseXML struct {
	XMLName xml.Name    `xml:"testcase"`
	Name    string      `xml:"name,attr"`
	Time    float64     `xml:"time,attr"`
	Failure *FailureXML `xml:"failure,omitempty"`
}

// <failure> XML element
type FailureXML struct {
	XMLName xml.Name `xml:"failure"`
	Message string   `xml:"message"`
}

// WriteXML writes a slice of TestSuites to a writer in XML format.
func WriteXML(suites []TestSuite, w io.Writer) error {
	suitesXML := TestSuitesXML{}
	for _, suite := range suites {
		suiteXML := TestSuiteXML{
			Name:  suite.Name,
			Time:  suite.Duration.Seconds(),
			Tests: len(suite.TestCases),
		}
		for _, t := range suite.TestCases {
			testXML := TestCaseXML{
				Name: t.Name,
				Time: t.Duration.Seconds(),
			}
			switch t.Status {
			case Failure:
				suiteXML.Failures += 1
				f := FailureXML{Message: t.Output.String()}
				testXML.Failure = &f
			case Skipped:
				suiteXML.Skipped += 1
			case Error:
				suiteXML.Errors += 1
			default:
				// do nothing
			}
			suiteXML.TestCases = append(suiteXML.TestCases, testXML)
		}
		suitesXML.TestSuites = append(suitesXML.TestSuites, suiteXML)
	}
	enc := xml.NewEncoder(w)
	err := enc.Encode(suitesXML)
	return err
}

func main() {
	suites, err := ParseOutput(os.Stdin)
	if err != nil {
		log.Fatal(err)
	}
	WriteXML(suites, os.Stdout)
}
