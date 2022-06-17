package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
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

func makeHTTPRequest(repo string) (string, error) {

	// set up request
	artifactory_url := "http://p3planrepo01.prod.phx3.gdg:8081/artifactory/api/storage/"
	values := url.Values{}
	u := os.Getenv("artifactoryUser") + ":" + os.Getenv("artifactoryPassword")
	values.Set("u", u)
	req, err := http.NewRequest("GET", artifactory_url+repo, strings.NewReader(values.Encode()))

	// client to do the request
	client := &http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		log.Fatal(err)
	}

	// read body text
	bodyText, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		log.Fatal(err)
	}

	return string(bodyText), err
}

func makeDownloadRequest(downloadUrl string) (*http.Response, error) {
	// set up request
	values := url.Values{}
	values.Set("u", "_scheduled:standard8")
	req, err := http.NewRequest("GET", downloadUrl, strings.NewReader(values.Encode()))

	// client to do the request
	client := &http.Client{
		Timeout: 60 * time.Second,
	}
	resp, err := client.Do(req)

	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	return resp, err
}

func downloadFile(filepath string, url string) (err error) {

	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Get the data
	resp, err := makeDownloadRequest(url)

	// Check server response
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	// Writer the body to file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
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

func downloadUriResponse(downloadPath string) {
	resp, err := makeHTTPRequest(downloadPath)
	if err != nil {
		log.Fatal(err)
	}

	response, parseErr := parseDownloadResponse(resp)
	if parseErr != nil {
		log.Fatal(parseErr)
	}
	fmt.Println(response.DownloadURI)
	downloadFile(downloadPath, response.DownloadURI)
}

/*
TODO:
1. Download maven-metadata.xml
2. Add a pause so it doesn't take down the server
3. Username and password from command line
*/
func downloadRepo(repo string) {
	resp, err := makeHTTPRequest(repo)
	if err != nil {
		log.Fatal(err)
	}

	response, parseErr := parseArtifactoryResponse(resp)
	if parseErr != nil {
		log.Fatal(parseErr)
	}
	// fmt.Println(resp.Children)
	for _, value := range response.Children {
		fileLocation := repo + value.URI
		if value.Folder == true {
			// fmt.Println(repo + value.URI)
			downloadRepo(repo + value.URI)
		} else {
			if err := os.MkdirAll(repo, os.ModePerm); err != nil {
				log.Fatal(err)
			}
			fmt.Println("Downloading", fileLocation)
			if _, err := os.Stat(fileLocation); os.IsNotExist(err) {
				downloadUriResponse(fileLocation)
			} else {
				fmt.Println("File already exists: ", fileLocation)
			}
		}
	}
}

func main() {
	// https://repo.int.dev-afternic.com/artifactory/webapp/simplebrowserroot.html?25
	localRepo := []string{
		"cloudera-local",
		"ext-release-local",
		"ext-snapshot-local",
		"libs-release-local",
		"libs-release-local-copy",
		"libs-snapshot-local",
		"old-java-build-dependencies",
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
	repo := []string{"libs-release-local"}

	downloadRepo(repo[0])
}
