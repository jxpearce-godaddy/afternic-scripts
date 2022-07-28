package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gtelang-godaddy/afternic-scripts/src/utils"
)

type ArtifactoryResponse struct {
	MetadataURI  string    `json:"metadataUri"`
	Repo         string    `json:"repo"`
	Path         string    `json:"path"`
	Created      time.Time `json:"created"`
	CreatedBy    string    `json:"createdBy"`
	LastModified time.Time `json:"lastModified"`
	ModifiedBy   string    `json:"modifiedBy"`
	LastUpdated  time.Time `json:"lastUpdated"`
	Children     []struct {
		URI    string `json:"uri"`
		Folder bool   `json:"folder"`
	} `json:"children"`
	URI string `json:"uri"`
}

type DownloadUriResponse struct {
	MetadataURI  string    `json:"metadataUri"`
	Repo         string    `json:"repo"`
	Path         string    `json:"path"`
	Created      time.Time `json:"created"`
	CreatedBy    string    `json:"createdBy"`
	LastModified time.Time `json:"lastModified"`
	ModifiedBy   string    `json:"modifiedBy"`
	LastUpdated  time.Time `json:"lastUpdated"`
	DownloadURI  string    `json:"downloadUri"`
	MimeType     string    `json:"mimeType"`
	Size         int       `json:"size"`
	Checksums    struct {
		Sha1 string `json:"sha1"`
		Md5  string `json:"md5"`
	} `json:"checksums"`
	OriginalChecksums struct {
		Sha1 string `json:"sha1"`
		Md5  string `json:"md5"`
	} `json:"originalChecksums"`
	URI string `json:"uri"`
}

func setUpRequestToOldArtifactory(path string) (*http.Request, error) {
	// set up request
	artifactoryUrl := "http://p3planrepo01.prod.phx3.gdg:8081/artifactory/api/storage/"
	values := url.Values{}
	u := os.Getenv("artifactoryUser") + ":" + os.Getenv("artifactoryPassword")
	values.Set("u", u)
	req, err := http.NewRequest("GET", artifactoryUrl+path, strings.NewReader(values.Encode()))
	return req, err
}

func makeHTTPRequest(repo string) (string, error) {

	req, err := setUpRequestToOldArtifactory(repo)

	if err != nil {
		panic(err)
	}

	// client to do the request
	client := &http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		fmt.Println(err)
		// log.Fatal(err)
	}

	// read body text
	bodyText, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		fmt.Println(err)
		// log.Fatal(err)
	}

	return string(bodyText), err
}

func downloadFile(filepath string, downloadUrl string) (err error) {
	fmt.Println("Downloading: ", filepath)
	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	req, err := setUpRequestToOldArtifactory(downloadUrl)

	if err != nil {
		panic(err)
	}
	// client to do the request
	client := &http.Client{
		Timeout: 60 * time.Second,
	}
	resp, err := client.Do(req)

	if err != nil {
		fmt.Println(err)
		// log.Fatal(err)
	}

	defer resp.Body.Close()
	// Check server response
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	// Writer the body to file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		fmt.Println(err)
		return err
	}

	return nil
}

func parseArtifactoryResponse(bodyText string) (ArtifactoryResponse, error) {
	var resp ArtifactoryResponse
	parseErr := json.Unmarshal([]byte(bodyText), &resp)
	return resp, parseErr
}

func parseDownloadResponse(bodyText string) (DownloadUriResponse, error) {
	var resp DownloadUriResponse
	parseErr := json.Unmarshal([]byte(bodyText), &resp)
	return resp, parseErr
}

func getDownloadUriResponse(downloadPath string) DownloadUriResponse {
	resp, err := makeHTTPRequest(downloadPath)
	if err != nil {
		fmt.Println(err)
		// log.Fatal(err)
	}

	response, parseErr := parseDownloadResponse(resp)
	if parseErr != nil {
		fmt.Println(parseErr)
		// log.Fatal(parseErr)
	}
	fmt.Println(response.DownloadURI)

	return response
}

/*
TODO: Download maven-metadata.xml if you're pushing to a maven artifactory?
*/
func downloadRepo(repo string) {
	resp, err := makeHTTPRequest(repo)
	if err != nil {
		fmt.Println(err)
		// log.Fatal(err)
	}

	response, parseErr := parseArtifactoryResponse(resp)
	if parseErr != nil {
		fmt.Println(parseErr)
		// log.Fatal(parseErr)
	}

	for _, value := range response.Children {
		fileLocation := repo + value.URI
		if value.Folder {
			// recurse
			downloadRepo(repo + value.URI)
		} else {
			// make directory
			if err := os.MkdirAll(repo, os.ModePerm); err != nil {
				log.Fatal(err)
			}
			response := getDownloadUriResponse(fileLocation)
			if _, err := os.Stat(fileLocation); os.IsNotExist(err) {
				downloadFile(fileLocation, response.DownloadURI)
			} else {
				fmt.Println("Check if file already exists & verify checksum: ", fileLocation)
				md5hash, _ := utils.GetCheckSum("md5", fileLocation)
				if response.Checksums.Md5 != md5hash {
					fmt.Println(response.Checksums.Md5)
					fmt.Println(md5hash)
					fmt.Println("Checksum did not match!")
					downloadFile(fileLocation, response.DownloadURI)
				}
			}
		}
	}
}

func generateArtifactoryRequest(path string, body io.Reader, sha1 string, sha256 string, md5 string) (*http.Request, error) {
	artifactoryUrl := "https://artifactory.secureserver.net/artifactory/generic-aftermarket-platform-dev-legacy-local/"
	req, err := http.NewRequest(http.MethodPut, artifactoryUrl+path, body)
	req.SetBasicAuth(os.Getenv("artifactoryUser"), os.Getenv("artifactoryPassword"))
	req.Header.Set("X-Checksum-Sha1", sha1)
	req.Header.Set("X-Checksum-Sha256", sha256)
	req.Header.Set("X-Checksum-Md5", md5)
	return req, err
}

func makeStatusCheck(path string, sha1 string, sha256 string, md5 string) (int, error) {
	// https://www.jfrog.com/confluence/display/JFROG/Artifactory+REST+API#ArtifactoryRESTAPI-DeployArtifactbyChecksum
	// You can browse here: https://artifactory.secureserver.net/ui/native/maven-aftermarket-platform-dev-legacy-local/rc-libs-local/
	req, _ := generateArtifactoryRequest(path, nil, sha1, sha256, md5)
	req.Header.Set("X-Checksum-Deploy", "true")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		// handle err
		fmt.Println("Error: ", err)
	}
	defer resp.Body.Close()
	return resp.StatusCode, err
}

func printResponseBody(resp *http.Response) {
	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(string(content))
}

func uploadArtifact(path string, sha1 string, sha256 string, md5 string) {
	fmt.Println("Uploading: ", path)
	file, err := os.Open(path)
	if err != nil {
		fmt.Println(err)
	}
	req, _ := generateArtifactoryRequest(path, file, sha1, sha256, md5)
	req.Header.Set("X-Checksum-Deploy", "false")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
	}
	// fmt.Println("Uploading status code: ", resp.StatusCode)
	// printResponseBody(resp)

	if resp.StatusCode != http.StatusCreated {
		printResponseBody(resp)
	}
	if err != nil {
		// handle err
		fmt.Println(err)
	}
	defer resp.Body.Close()
}

func downloadFromP3Plan() {
	// https://repo.int.dev-afternic.com/artifactory/webapp/simplebrowserroot.html?25
	// All dependencies are maven-2-default
	localRepo := []string{
		"cloudera-local",
		"ext-release-local",
		"ext-snapshot-local",
		"libs-release-local",
		"libs-release-local-copy",
		"libs-snapshot-local",
		"old-java-build-dependencies", // gradle
		"plugins-release-local",
		"plugins-snapshot-local",
		"products-release-local",
		"products-snapshot-local",
		"rc-libs-local",
		"rc-plugins-local",
		"rc-products-local"}

	repositoryCache := []string{
		"codehaus-cache",
		"google-code-cache",
		"gradle-libs-cache",
		"gradle-plugins-cache",
		"java.net.m1-cache",
		"java.net.m2-cache",
		"jboss-cache",
		"jfrog-libs-cache",
		"jfrog-plugins-cache",
		"repo1-cache",
		"spring-milestone-cache",
		"spring-release-cache",
		"codehaus",
		"google-code",
		"gradle-libs",
		"gradle-plugins",
		"java.net.m1",
		"java.net.m2",
		"jboss",
		"jfrog-libs",
		"jfrog-plugins",
		"repo1",
		"spring-milestone",
		"spring-release"}

	repositoryVirtual := []string{
		"libs-release",
		"libs-snapshot",
		"plugins-release",
		"plugins-snapshot",
		"remote-repos",
		"repo"}
	fmt.Println("Repository cache items: ", len(repositoryCache))
	fmt.Println("Repository virtual items: ", len(repositoryVirtual))
	fmt.Println("Local repo items: ", len(localRepo))

	for _, repo := range repositoryCache {
		downloadRepo(repo)
	}

	for _, repo := range repositoryVirtual {
		downloadRepo(repo)
	}

	for _, repo := range localRepo {
		downloadRepo(repo)
	}
}

func uploadFolderToArtifactory(folder string) {

	files := []string{}
	dirs := []string{}

	filepath.WalkDir(folder, func(path string, di fs.DirEntry, err error) error {
		info, _ := os.Stat(path)
		if !info.IsDir() {
			files = append(files, path)
		} else {
			dirs = append(dirs, path)
		}
		return nil
	})

	for _, path := range files {
		_sha1, _ := utils.GetCheckSum("sha1", path)
		_sha256, _ := utils.GetCheckSum("sha256", path)
		_md5, _ := utils.GetCheckSum("md5", path)
		// Deploy only if checksum does not match. If checksum does not match, artifactory throws 404
		statusCode, err := makeStatusCheck(path, _sha1, _sha256, _md5)
		if err != nil {
			fmt.Println("Error in status check", err)
		}
		fmt.Println("Visited:", path, statusCode)
		if statusCode == http.StatusNotFound {
			fmt.Println("Uploading for path", path, statusCode)
			uploadArtifact(path, _sha1, _sha256, _md5)
		}
	}
}

func uploadToGDArtifactory() {
	folders := []string{
		"rc-libs-local",
		"libs-release",
		"libs-snapshot",
		"plugins-release",
		"rc-products-local",
		"libs-release-local",
		"libs-snapshot-local",
		"plugins-snapshot",
		"remote-repos",
		"gradle-libs-cache",
		"libs-release-local-copy",
		"repo",
	}

	var wg sync.WaitGroup
	startTime := time.Now()
	fmt.Printf("Start time %s", startTime)

	for _, folder := range folders {
		wg.Add(1)
		go func(folder string) {
			uploadFolderToArtifactory(folder)
			wg.Done()
		}(folder)
	}
	wg.Wait()
	endTime := time.Now()
	fmt.Printf("end time %s", endTime)
}

// Migrate from: http://p3planrepo01.prod.phx3.gdg:8081/
// to: https://artifactory.secureserver.net/artifactory/webapp/#/artifacts/browse/tree/General/generic-aftermarket-platform-dev-legacy-local

// Uploading to a generic repo since if it's maven2 then pom consistency checks are enforced and I would need multiple repos instead of a single one.
// Example: rc-libs-local and lib-release-snapshot would be under a different repo.
// Other option would be figure out how to rebuild the pom.xml for maven repos (but would also mean we need do it for lot of jars)
func main() {

	args := os.Args

	if args[1] == "download" {
		fmt.Println("Downloading from p3plan.")
		downloadFromP3Plan()
	} else if args[1] == "upload" {
		fmt.Println("Uploading to secureserver artifactory.")
		uploadToGDArtifactory()
	}
}
