package main

import (
	"crypto/md5"
	"encoding/hex"
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

func setUpRequestToArtifactory(path string) (*http.Request, error) {
	// set up request
	artifactoryUrl := "http://p3planrepo01.prod.phx3.gdg:8081/artifactory/api/storage/"
	values := url.Values{}
	u := os.Getenv("artifactoryUser") + ":" + os.Getenv("artifactoryPassword")
	values.Set("u", u)
	req, err := http.NewRequest("GET", artifactoryUrl+path, strings.NewReader(values.Encode()))
	return req, err
}

func makeHTTPRequest(repo string) (string, error) {

	req, err := setUpRequestToArtifactory(repo)

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

// TODO: turn into a method without throwing
// func makeDownloadRequest(downloadUrl string) (*http.Response, error) {
// 	// set up request

// 	return resp, err
// }

func downloadFile(filepath string, downloadUrl string) (err error) {
	fmt.Println("Downloading: ", filepath)
	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	req, err := setUpRequestToArtifactory(downloadUrl)

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
TODO: 1. Download maven-metadata.xml
*/
func getMd5Checksum(filepath string) string {
	file, err := os.Open(filepath)

	if err != nil {
		panic(err)
	}

	defer file.Close()

	hash := md5.New()
	_, err = io.Copy(hash, file)

	if err != nil {
		panic(err)
	}
	_hash := hash.Sum(nil)
	return hex.EncodeToString(_hash[:])
}

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
		if value.Folder == true {
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
				md5hash := getMd5Checksum(fileLocation)
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

func main() {
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
