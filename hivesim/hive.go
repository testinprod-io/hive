package hivesim

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
)

// Simulation wraps the simulation HTTP API provided by hive.
type Simulation struct {
	url string
}

// New looks up the hive host URI using the HIVE_SIMULATOR environment variable
// and connects to it. It will panic if HIVE_SIMULATOR is not set.
func New() *Simulation {
	simulator, isSet := os.LookupEnv("HIVE_SIMULATOR")
	if !isSet {
		panic("HIVE_SIMULATOR environment variable not set")
	}
	return &Simulation{url: simulator}
}

// NewAt creates a simulation connected to the given API endpoint. You'll will rarely need
// to use this. In simulations launched by hive, use New() instead.
func NewAt(url string) *Simulation {
	return &Simulation{url: url}
}

// GetClientEnode returns the enode URL of a running client.
func (sim *Simulation) GetClientEnode(testSuite SuiteID, test TestID, node string) (*string, error) {
	resp, err := http.Get(fmt.Sprintf("%s/testsuite/%s/test/%s/node/%s", sim.url, testSuite.String(), test.String(), node))
	if err != nil {
		return nil, err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	res := strings.TrimRight(string(body), "\r\n")
	return &res, nil
}

// EndTest finishes the test case, cleaning up everything, logging results, and returning
// an error if the process could not be completed.
func (sim *Simulation) EndTest(testSuite SuiteID, test TestID, summaryResult TestResult, clientResults map[string]TestResult) error {
	// post results (which deletes the test case - because DELETE message body is not always supported)
	summaryResultData, err := json.Marshal(summaryResult)
	if err != nil {
		return err
	}
	clientResultData, err := json.Marshal(clientResults)
	if err != nil {
		return err
	}

	vals := make(url.Values)
	vals.Add("summaryresult", string(summaryResultData))
	vals.Add("clientresults", string(clientResultData))
	_, err = wrapHTTPErrorsPost(fmt.Sprintf("%s/testsuite/%s/test/%s", sim.url, testSuite.String(), test.String()), vals)
	return err
}

// StartTestSuite signals the start of a test suite.
func (sim *Simulation) StartTestSuite(name, description, simlog string) (SuiteID, error) {
	vals := make(url.Values)
	vals.Add("name", name)
	vals.Add("description", description)
	vals.Add("simlog", simlog)
	idstring, err := wrapHTTPErrorsPost(fmt.Sprintf("%s/testsuite", sim.url), vals)
	if err != nil {
		return 0, err
	}
	id, err := strconv.Atoi(idstring)
	if err != nil {
		return 0, err
	}
	return SuiteID(id), nil
}

// EndTestSuite signals the end of a test suite.
func (sim *Simulation) EndTestSuite(testSuite SuiteID) error {
	req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/testsuite/%s", sim.url, testSuite.String()), nil)
	if err != nil {
		return err
	}
	_, err = http.DefaultClient.Do(req)
	return err
}

// StartTest starts a new test case, returning the testcase id as a context identifier.
func (sim *Simulation) StartTest(testSuite SuiteID, name string, description string) (TestID, error) {
	vals := make(url.Values)
	vals.Add("name", name)
	vals.Add("description", description)

	idstring, err := wrapHTTPErrorsPost(fmt.Sprintf("%s/testsuite/%s/test", sim.url, testSuite.String()), vals)
	if err != nil {
		return 0, err
	}
	testID, err := strconv.Atoi(idstring)
	if err != nil {
		return 0, err
	}
	return TestID(testID), nil
}

// GetClientTypes returns all client types available to this simulator run. This depends on
// both the available client set and the command line filters.
func (sim *Simulation) GetClientTypes() (availableClients []string, err error) {
	resp, err := http.Get(fmt.Sprintf("%s/clients", sim.url))
	if err != nil {
		return nil, err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(body, &availableClients)
	if err != nil {
		return nil, err
	}
	return
}

// GetNode starts a new node (or other container) with the specified parameters. One
// parameter must be named CLIENT and should contain one of the client types from
// GetClientTypes. The input is used as environment variables in the new container.
// Returns container id and ip.
func (sim *Simulation) GetNode(testSuite SuiteID, test TestID, parameters map[string]string, initFiles map[string]string) (string, net.IP, *string, error) {
	// vals := make(url.Values)
	// for k, v := range parameters {
	// 	vals.Add(k, v)
	// }
	//vals.Add("testcase", test.String())
	//	parameters["testcase"] = test.String()
	data, err := postWithFiles(fmt.Sprintf("%s/testsuite/%s/test/%s/node", sim.url, testSuite.String(), test.String()), parameters, initFiles)
	if err != nil {
		return "", nil, nil, err
	}
	if idip := strings.Split(data, "@"); len(idip) > 1 {
		return idip[0], net.ParseIP(idip[1]), &idip[2], nil
	}
	return data, net.IP{}, nil, fmt.Errorf("no ip address returned: %v", data)
}

// GetPseudo starts a new pseudo-client with the specified parameters One parameter must be
// named CLIENT and should contain one of the returned client types from GetClientTypes
// The input is used as environment variables in the new container.
//
// Returns container id, ip and mac.
func (sim *Simulation) GetPseudo(testSuite SuiteID, test TestID, parameters map[string]string) (string, net.IP, *string, error) {
	vals := make(url.Values)
	for k, v := range parameters {
		vals.Add(k, v)
	}
	data, err := wrapHTTPErrorsPost(fmt.Sprintf("%s/testsuite/%s/test/%s/pseudo", sim.url, testSuite.String(), test.String()), vals)
	if err != nil {
		return "", nil, nil, err
	}
	if idip := strings.Split(data, "@"); len(idip) > 1 {
		return idip[0], net.ParseIP(idip[1]), &idip[2], nil
	}
	return data, net.IP{}, nil, fmt.Errorf("no ip address returned: %v", data)
}

// KillNode signals to the host that the node is no longer required.
func (sim *Simulation) KillNode(testSuite SuiteID, test TestID, nodeid string) error {
	req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/testsuite/%s/test/%s/node/%s", sim.url, testSuite.String(), test.String(), nodeid), nil)
	if err != nil {
		return err
	}
	_, err = http.DefaultClient.Do(req)
	return err
}

func postWithFiles(url string, values map[string]string, files map[string]string) (string, error) {
	var err error

	// make a dictionary of readers
	formValues := make(map[string]io.Reader)
	for key, s := range values {
		formValues[key] = strings.NewReader(s)
	}
	for key, filename := range files {
		filereader, err := os.Open(filename)
		if err != nil {
			return "", err
		}
		//fi, err := filereader.Stat()
		formValues[key] = filereader
	}

	// send them
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	for key, r := range formValues {
		var fw io.Writer
		if x, ok := r.(io.Closer); ok {
			defer x.Close()
		}
		if x, ok := r.(*os.File); ok {
			if fw, err = w.CreateFormFile(key, x.Name()); err != nil {
				return "", err
			}
		} else {
			if fw, err = w.CreateFormField(key); err != nil {
				return "", err
			}
		}
		if _, err = io.Copy(fw, r); err != nil {
			return "", err
		}
	}

	// this must be closed or the request will be missing the terminating boundary
	w.Close()

	// Can't use http.PostForm because we need to change the content header
	req, err := http.NewRequest("POST", url, &b)
	if err != nil {
		return "", err
	}
	// Set the content type, this will contain the boundary.
	req.Header.Set("Content-Type", w.FormDataContentType())

	// Submit the request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode >= 200 && resp.StatusCode <= 300 {
		return string(body), nil
	}
	return "", fmt.Errorf("request failed (%d): %v", resp.StatusCode, string(body))
}

// wrapHttpErrorsPost wraps http.PostForm to convert responses that are not 200 OK into errors
func wrapHTTPErrorsPost(url string, data url.Values) (string, error) {
	resp, err := http.PostForm(url, data)
	if err != nil {
		return "", err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode >= 200 && resp.StatusCode <= 300 {
		return string(body), nil
	}
	return "", fmt.Errorf("request failed (%d): %v", resp.StatusCode, string(body))
}
