package server

import (
	"SalesforceGit/auth/goth"
	"SalesforceGit/auth/gothic"
	"SalesforceGit/cdcSubscribe/common"
	"SalesforceGit/cdcSubscribe/subscribe"
	"SalesforceGit/db"
	"SalesforceGit/soql"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/url"
	"reflect"
	"strings"
)

func Logout(res http.ResponseWriter, req *http.Request) {
	gothic.Logout(res, req)
	res.Header().Set("Location", "/login")
	res.WriteHeader(http.StatusTemporaryRedirect)
}

func SalesforceCallback(res http.ResponseWriter, req *http.Request) {
	checkAuth(res, req)
	http.Redirect(res, req, "/index", http.StatusSeeOther)
}

func AuthSalesforce(res http.ResponseWriter, req *http.Request) {
	// try to get the user without re-authenticating
	// Now you have access to the value of the "provider" parameter

	// try to get the user without re-authenticating
	if _, err := gothic.CompleteUserAuth(res, req); err == nil {
		http.Redirect(res, req, "/index", http.StatusSeeOther)

	} else {
		gothic.BeginAuthHandler(res, req)
	}
}

func Index(res http.ResponseWriter, req *http.Request) {
	user := checkAuth(res, req)
	t, err := template.ParseFiles("./templates/index.html")
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	collections, err := db.GetListOfCollections()
	var cdcCollections []string
	for _, el := range collections {
		if strings.Contains(el, "ChangeEvent") {
			var found bool
			el, found = strings.CutPrefix(el, "/data/")
			if !found {
				break
			}
			el, found = strings.CutSuffix(el, "ChangeEvent")
			if !found {
				break
			}
			cdcCollections = append(cdcCollections, el)

		}
	}

	type model struct {
		User        goth.User
		Collections []string
	}

	data := model{
		User:        user,
		Collections: cdcCollections,
	}

	if err := t.Execute(res, data); err != nil {
		//http.Error(res, err.Error(), http.StatusInternalServerError)
		fmt.Println(err)
		return
	}
}

func Login(res http.ResponseWriter, req *http.Request) {
	// delete all sessions?
	//gothic.Logout(res, req)

	t, err := template.ParseFiles("./templates/login.html")
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := t.Execute(res, nil); err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
}

func SetupSubscribe(res http.ResponseWriter, req *http.Request) {
	// Check user authentication
	user := checkAuth(res, req)

	// Parse the template file
	t, err := template.ParseFiles("./templates/subscribe.html")

	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	subscribeValues, err := db.GetSubscribeValues()

	if err != nil {
		fmt.Println(err)
	}

	type Model struct {
		User            goth.User
		SubscribeValues common.IntegrationUserValues
	}

	model := Model{
		User:            user,
		SubscribeValues: subscribeValues,
	}
	// Execute the template with the user data
	err = t.Execute(res, model)
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
}

func ConfigureSubscribeData(res http.ResponseWriter, req *http.Request) {
	checkAuth(res, req)
	if req.Method != http.MethodPost {
		http.Error(res, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read the request body
	body, err := io.ReadAll(req.Body)
	if err != nil {
		http.Error(res, "Error reading request body", http.StatusInternalServerError)
		return
	}
	// Parse the form data from the request body
	var formData common.IntegrationUserValues
	err = json.Unmarshal(body, &formData)
	if err != nil {
		http.Error(res, "Error parsing form data", http.StatusBadRequest)
		return
	}

	// Print or process the form data as needed
	err = db.UpdateSubscribeValues(formData)

	if err != nil {
		fmt.Println(err)
	}

	err = subscribe.UpdateSubscribe()

	if err != nil {
		fmt.Println(err)
	}
	// Respond with a success message
	res.WriteHeader(http.StatusOK)
}

func ViewObjects(res http.ResponseWriter, req *http.Request) {
	user := checkAuth(res, req)
	model := req.URL.Path[len("/viewEvents/"):]
	fmt.Println(model)
	objectsBson, err := db.GetChangedObjects(model)
	if err != nil {
		log.Println(err)
	}
	objects := db.SetupObjects(objectsBson)

	type ObjectOutput struct {
		RecordId         string
		RecordName       string
		CommitUserName   string
		LastModifiedDate string
	}
	var objectsStruct []ObjectOutput
	var userIds []string
	var recordIds []string
	for _, el := range objects {
		userIds = append(userIds, el.UserId)
		recordIds = append(recordIds, el.RecordId)
	}
	userNames, err := soql.GetNamesById("User", userIds)

	if err != nil {
		log.Println(err)
	}
	recordNames, err := soql.GetNamesById(model, recordIds)

	if err != nil {
		log.Println(err)
	}
	for _, el := range objects {
		var obj ObjectOutput
		for _, receivedRecordField := range recordNames {
			if el.RecordId == receivedRecordField.Id {
				obj.RecordId = el.RecordId
				obj.RecordName = receivedRecordField.Name
			}
		}
		for _, receivedUserField := range userNames {
			if el.UserId == receivedUserField.Id {
				obj.CommitUserName = receivedUserField.Name
			}
		}

		obj.LastModifiedDate, err = db.FormatDate(el.Date)
		objectsStruct = append(objectsStruct, obj)
	}

	type dataStruct struct {
		ModelName   string
		User        goth.User
		ObjectsData []ObjectOutput
	}

	data := dataStruct{
		ModelName:   model,
		User:        user,
		ObjectsData: objectsStruct,
	}

	funcs := template.FuncMap{
		"isArray": func(value interface{}) bool {
			return reflect.TypeOf(value).Kind() == reflect.Slice
		},
		"sub": func(a, b int) int {
			return a - b
		},
	}
	t, err := template.New("viewObjects.html").Funcs(funcs).ParseFiles("./templates/viewObjects.html")
	fmt.Println(err)
	err = t.Execute(res, data)
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
}
func ViewEventsFromCollection(res http.ResponseWriter, req *http.Request) {
	user := checkAuth(res, req)
	exactData := req.URL.Path[len("/viewEvents/"):]
	urlValues := strings.Split(exactData, "/")
	// urlValues[0] is model, urlValues[1] is id of record
	events, err := db.GetEventsById(urlValues[0], urlValues[1]) // model and id as a params
	if err != nil {
		fmt.Println(err)
	}
	changeEventsStruct := db.SetupEventForOutput(events)

	type dataStruct struct {
		ModelName       string
		User            goth.User
		ChangeEventData []db.ChangeEvent
		Id              string
		EntityName      string
	}

	id := changeEventsStruct[0].EventHeader.RecordIds[0]
	name := changeEventsStruct[0].EventHeader.EntityName
	data := dataStruct{ModelName: urlValues[0], User: user, ChangeEventData: changeEventsStruct, Id: id, EntityName: name}
	funcs := template.FuncMap{
		"isArray": func(value interface{}) bool {
			return reflect.TypeOf(value).Kind() == reflect.Slice
		},
	}
	//t, err := template.ParseFiles("./templates/viewEvents.html")
	t, err := template.New("viewEvents.html").Funcs(funcs).ParseFiles("./templates/viewEvents.html")
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	if err = t.Execute(res, data); err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
}

func ConfigServer(res http.ResponseWriter, req *http.Request) {
	// Check user authentication
	user := checkAuth(res, req)

	// Parse the template file
	t, err := template.ParseFiles("./templates/configServer.html")

	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	subscribeValues, err := db.GetSubscribeValues()

	if err != nil {
		fmt.Println(err)
	}

	type Model struct {
		User            goth.User
		SubscribeValues common.IntegrationUserValues
	}

	model := Model{
		User:            user,
		SubscribeValues: subscribeValues,
	}
	err = t.Execute(res, model)
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
}

func SetupDateFormat(res http.ResponseWriter, req *http.Request) {
	checkAuth(res, req)
	if req.Method != http.MethodPost {
		http.Error(res, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read the request body
	body, err := io.ReadAll(req.Body)
	fmt.Println(string(body))
	formData, err := url.ParseQuery(string(body))
	if err != nil {
		log.Println(err)
	}

	common.DateFormatValues = common.DateFormat{
		Timezone:   formData.Get("timezone"),
		DateFormat: formData.Get("dateFormat"),
		TimeFormat: formData.Get("timeFormat"),
	}
	fmt.Println(common.DateFormatValues)
	if err != nil {
		http.Error(res, "Error parsing form data", http.StatusBadRequest)
		return
	}

}

func checkAuth(res http.ResponseWriter, req *http.Request) goth.User {
	user, err := gothic.CompleteUserAuth(res, req)
	if err != nil {
		// redirect to login page
		res.Header().Set("Location", "/login")
		res.WriteHeader(http.StatusTemporaryRedirect)
		//fmt.Println(err)
	}
	common.AccessToken = user.AccessToken
	return user
}
