package go-sharefile

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var (
	// token used for accessing auth data once initialised,  without needing to call a function
	token        map[string]string
	uploadConfig map[string]string
)

// Generic struct for the top level json object response, contains additional childObject struct as an array for an arbitrary number of children.
type baseObject struct {
	ID           string        `json:"id"`
	CreationDate string        `json:"CreationDate"`
	Name         string        `json:"Name"`
	Children     []childObject `json:"Children"`
}

type childObject struct {
	ID           string `json:"id"`
	CreationDate string `json:"CreationDate"`
	Name         string `json:"Name"`
}

// Struct for use in folder POST activities
type folderBody struct {
	Name        string
	Description string
}

// Struct for use in user POST activities
type userBody struct {
	Email             string
	FirstName         string
	LastName          string
	Company           string
	ClientPassword    string
	CanResetPassword  bool
	CanViewMySettings bool
}

// Struct for use in client POST/GET activities
type valueBody struct {
	Client []clientBody `json:"value"`
}

type clientBody struct {
	ID    string `json:"Id"`
	Email string `json:"Email"`
}

// Authenticate authenticates against the given instance, and should be the first function to be run, as it prepares auth for the entire package.
func Authenticate(hostname, clientID, clientSecret, username, password string) {

	uriPath := "/oauth/token"

	message := url.Values{
		"grant_type":    {"password"},
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"username":      {username},
		"password":      {password},
	}

	resp, err := http.PostForm(fmt.Sprintf("%s%s", hostname, uriPath), message)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	var tokenResponse map[string]string

	json.NewDecoder(resp.Body).Decode(&tokenResponse)

	token = tokenResponse

}

// Returns ShareFile authorization header, internal package use.
func getAuthorizationHeader() string {
	return fmt.Sprintf("Bearer %s", token["access_token"])
}

// Returns ShareFile API hostname, internal package use.
func getHostname() string {
	return fmt.Sprintf("%s.sf-api.com", token["subdomain"])
}

// GetRoot returns the root level Item for the provided user.
func GetRoot(getChildren ...bool) {
	uriPath := "/sf/v3/Items(allshared)"
	if getChildren[0] {
		uriPath = fmt.Sprintf("%s?$expand=Children", uriPath)
	}

	client := http.Client{}

	req, err := http.NewRequest("GET", fmt.Sprintf("https://%s%s", getHostname(), uriPath), nil)
	if err != nil {
		log.Fatalln(err)
	}

	req.Header.Add("Authorization", getAuthorizationHeader())

	resp, err := client.Do(req)
	if err != nil {
		log.Fatalln(err)
	}
	defer resp.Body.Close()

	// Load Json into structs
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	items := baseObject{}
	err = json.Unmarshal(body, &items)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("%s %s %s\n", items.ID, items.CreationDate, items.Name)
	if len(items.Children) != 0 {
		for i := range items.Children {
			fmt.Printf("%s %s %s\n", items.Children[i].ID, items.Children[i].CreationDate, items.Children[i].Name)
		}

	}
}

// GetItemByID returns a single item, for which the ID is provided.
func GetItemByID(itemID string) {
	uriPath := fmt.Sprintf("/sf/v3/Items(%s)", itemID)

	client := http.Client{}
	req, err := http.NewRequest("GET", fmt.Sprintf("https://%s%s", getHostname(), uriPath), nil)
	if err != nil {
		log.Fatalln(err)
	}

	req.Header.Add("Authorization", getAuthorizationHeader())

	resp, err := client.Do(req)
	if err != nil {
		log.Fatalln(err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	items := baseObject{}
	err = json.Unmarshal(body, &items)
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Printf("%v %v %v\n", items.ID, items.CreationDate, items.Name)
}

// GetFolderWithQueryParameters gets a folder using some of the common query parameters that are available.
// This will add the expand, select parameters. The following are used:
//
// expand=Children to get any Children of the folder
// select=Id,Name,Children/Id,Children/Name,Children/CreationDate to get the Id, Name of the folder
// and the Id, Name, CreationDae of any Children
func GetFolderWithQueryParameters(itemID string) {
	uriPath := fmt.Sprintf("/sf/v3/Items(%s)?$expand=Children&$select=Id,Name,Children/Id,Children/Name,Children/CreationDate", itemID)
	fmt.Printf("GET %s%s", getHostname(), uriPath)
	client := http.Client{}

	req, err := http.NewRequest("GET", fmt.Sprintf("https://%s%s", getHostname(), uriPath), nil)
	if err != nil {
		log.Fatalln(err)
	}

	req.Header.Add("Authorization", getAuthorizationHeader())

	resp, err := client.Do(req)
	if err != nil {
		log.Fatalln(err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Println(resp.Status)

	items := baseObject{}
	err = json.Unmarshal(body, &items)
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Printf("%s %s %s\n", items.ID, items.CreationDate, items.Name)
	if len(items.Children) != 0 {
		for i := range items.Children {
			fmt.Printf("%s %s %s\n", items.Children[i].ID, items.Children[i].CreationDate, items.Children[i].Name)
		}
	}
}

// CreateFolder creates a new folder in the given parent folder.
func CreateFolder(parentID string, name string, description string) {
	folder := folderBody{
		Name:        name,
		Description: description,
	}

	f, err := json.Marshal(folder)
	if err != nil {
		log.Fatalln(err)
	}

	var dataBytes bytes.Buffer
	dataBytes.Write(f)

	uriPath := fmt.Sprintf("/sf/v3/Items(%s)/Folder", parentID)
	fmt.Printf("POST %s%s", getHostname(), uriPath)
	client := http.Client{}

	req, err := http.NewRequest("POST", fmt.Sprintf("https://%s%s", getHostname(), uriPath), bytes.NewBuffer(dataBytes.Bytes()))
	if err != nil {
		log.Fatalln(err)
	}

	req.Header.Add("Authorization", getAuthorizationHeader())
	req.Header.Add("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		log.Fatalln(err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Println(resp.Status)

	items := baseObject{}
	err = json.Unmarshal(body, &items)
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Printf("Created folder %v", items.ID)

}

// UpdateItem updates the name and description of an item.
func UpdateItem(itemID string, name string, description string) {

	folder := folderBody{
		Name:        name,
		Description: description,
	}

	f, err := json.Marshal(folder)
	if err != nil {
		log.Fatalln(err)
	}

	var dataBytes bytes.Buffer
	dataBytes.Write(f)

	uriPath := fmt.Sprintf("/sf/v3/Items(%s)/Folder", itemID)
	fmt.Printf("PATCH %s%s", getHostname(), uriPath)
	client := http.Client{}

	req, err := http.NewRequest("PATCH", fmt.Sprintf("https://%s%s", getHostname(), uriPath), bytes.NewBuffer(dataBytes.Bytes()))
	if err != nil {
		log.Fatalln(err)
	}

	req.Header.Add("Authorization", getAuthorizationHeader())
	req.Header.Add("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		log.Fatalln(err)
	}
	defer resp.Body.Close()

	fmt.Println(resp.Status)
}

// DeleteItem deletes and item by id.
func DeleteItem(itemID string) {
	uriPath := fmt.Sprintf("/sf/v3/Items(%s)", itemID)
	fmt.Printf("DELETE %s%s", getHostname(), uriPath)
	client := http.Client{}

	req, err := http.NewRequest("DELETE", fmt.Sprintf("https://%s%s", getHostname(), uriPath), nil)

	if err != nil {
		log.Fatalln(err)
	}

	req.Header.Add("Authorization", getAuthorizationHeader())

	resp, err := client.Do(req)
	if err != nil {
		log.Fatalln(err)
	}
	defer resp.Body.Close()

	// Should expect 204 No Content message here
	fmt.Println(resp.Status)

}

// DownloadItem downloads a single item. If downloading a folder the localPath name should end in .zip.
func DownloadItem(itemID string, localPath string) {
	uriPath := fmt.Sprintf("/sf/v3/Items(%s)/Download", itemID)
	fmt.Printf("GET %s%s\n", getHostname(), uriPath)
	client := http.Client{}

	req, err := http.NewRequest("GET", fmt.Sprintf("https://%s%s", getHostname(), uriPath), nil)
	if err != nil {
		log.Fatalln(err)
	}

	req.Header.Add("Authorization", getAuthorizationHeader())

	resp, err := client.Do(req)
	if err != nil {
		log.Fatalln(err)
	}
	defer resp.Body.Close()

	out, err := os.Create(localPath)
	if err != nil {
		log.Fatalln(err)
	}
	defer out.Close()

	if _, err := io.Copy(out, resp.Body); err != nil {
		log.Fatalln(err)
	}
}

// UploadFile uploads a File using the standard upload method with a multipart/form mime encoded POST
func UploadFile(localPath string, folderID string) int {
	if token["access_token"] == "" {
		log.Println("ShareFile token not obtained")
	}

	client := http.Client{}
	uriPath := fmt.Sprintf("/sf/v3/Items(%s)/Upload", folderID)

	req, err := http.NewRequest("GET", fmt.Sprintf("%s%s", getHostname(), uriPath), nil)
	if err != nil {
		log.Fatalln(err)
	}

	req.Header.Add("Authorization", getAuthorizationHeader())

	resp, err := client.Do(req)
	if err != nil {
		log.Fatalln(err)
	}
	defer resp.Body.Close()

	json.NewDecoder(resp.Body).Decode(&uploadConfig)

	var uploadResponse *http.Response
	if len(uploadConfig["ChunkUri"]) > 0 {
		uploadResponse = multipartFormPostUpload(uploadConfig["ChunkUri"], localPath)
	} else {
		log.Print("No Upload URL received")
	}

	return uploadResponse.StatusCode

}

// Does a multipart form post upload of a file to a url, internal package use.
func multipartFormPostUpload(u string, fp string) *http.Response {
	filename := filepath.Base(fp)
	data := strings.Builder{}
	headers := make(map[string]string)

	fileContent, err := getContentType(fp)
	if err != nil {
		fileContent = "application/octet-stream"
	}

	file, err := ioutil.ReadFile(fp)
	if err != nil {
		log.Fatalln(err)
	}

	boundary := fmt.Sprintf("----------%v", time.Now().Unix())
	headers["Content-Type"] = fmt.Sprintf("multipart/form-data; boundary=%v", boundary)

	// To note if copying/converting this data structure, carriage returns are required. Interpret it exactly as written.
	data.WriteString(fmt.Sprintf("--%v\r\n", boundary))
	data.WriteString(fmt.Sprintf("Content-Disposition: form-data; name=\"File1\"; filename=\"%v\"\r\n", filename))
	data.WriteString(fmt.Sprintf("Content-Type: %v\r\n\r\n", fileContent))
	data.Write(file)
	data.WriteString(fmt.Sprintf("\r\n--%v--\r\n", boundary))

	var dataStr bytes.Buffer

	dataStr.WriteString(data.String())

	uri, _ := url.Parse(u)

	req, err := http.NewRequest("POST", uri.String(), bytes.NewBuffer(dataStr.Bytes()))
	if err != nil {
		log.Fatalln("Request failed")
	}

	headers["Content-Length"] = string(dataStr.Len())

	for hdrName, hdrValue := range headers {
		req.Header.Add(hdrName, string(hdrValue))
	}

	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		log.Fatalln(err)
	}
	defer resp.Body.Close()

	return resp
}

func getContentType(f string) (string, error) {

	buffer := make([]byte, 512)

	buffer, err := ioutil.ReadFile(f)
	if err != nil {
		return "", err
	}

	contentType := http.DetectContentType(buffer)
	return contentType, nil
}

// GetClients gets the client users in the account.
func GetClients() {
	uriPath := "/sf/v3/Accounts/Clients"
	fmt.Printf("GET %s%s\n", getHostname(), uriPath)
	client := http.Client{}

	req, err := http.NewRequest("GET", fmt.Sprintf("https://%v%v", getHostname(), uriPath), nil)
	if err != nil {
		log.Fatalln(err)
	}

	req.Header.Add("Authorization", getAuthorizationHeader())

	resp, err := client.Do(req)
	if err != nil {
		log.Fatalln(err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln(err)
	}

	items := valueBody{}
	err = json.Unmarshal(body, &items)
	if err != nil {
		log.Fatalln(err)
	}

	for i := range items.Client {
		fmt.Printf("%s %s\n", items.Client[i].ID, items.Client[i].Email)
	}
}

// CreateClient creates a client user in the account
func CreateClient(email, firstname, lastname, company, clientpassword string, canresetpassword, canviewmysettings bool) {
	user := userBody{
		Email:             email,
		FirstName:         firstname,
		LastName:          lastname,
		Company:           company,
		ClientPassword:    clientpassword,
		CanResetPassword:  canresetpassword,
		CanViewMySettings: canviewmysettings,
	}

	u, err := json.Marshal(user)
	if err != nil {
		log.Fatalln(err)
	}

	var dataBytes bytes.Buffer
	dataBytes.Write(u)

	uriPath := "/sf/v3/Users"
	fmt.Printf("POST %s%s", getHostname(), uriPath)
	client := http.Client{}

	req, err := http.NewRequest("POST", fmt.Sprintf("https://%s%s", getHostname(), uriPath), bytes.NewBuffer(dataBytes.Bytes()))
	if err != nil {
		log.Fatalln(err)
	}

	req.Header.Add("Authorization", getAuthorizationHeader())
	req.Header.Add("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		log.Fatalln(err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln(err)
	}

	items := baseObject{}
	err = json.Unmarshal(body, &items)
	if err != nil {
		log.Fatalln(err)
	}

	// Zero error handling
	fmt.Println(resp.Status)
	fmt.Printf("Created Client %s\n", items.ID)
}
