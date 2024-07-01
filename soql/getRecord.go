package soql

import (
	"SalesforceGit/cdcSubscribe/common"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/tidwall/gjson"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type RecordIdName struct {
	Id   string
	Name string
}

func GetNamesById(model string, ids []string) ([]RecordIdName, error) {

	query := fmt.Sprintf("SELECT %s FROM %s WHERE Id in ('%s')", "Name,Id", model, strings.Join(ids, "','"))
	salesForceUrl, _ := url.Parse("https://great-codey-488635-dev-ed.my.salesforce.com/services/data/v54.0/query/")
	q := url.Values{}
	q.Add("q", query)
	salesForceUrl.RawQuery = q.Encode()
	req, err := http.NewRequest("GET", salesForceUrl.String(), nil)
	if err != nil {
		return nil, errors.New("error creating url for soql request")
	}

	req.Header.Add("Authorization", "Bearer "+common.AccessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var data map[string]json.RawMessage

	if err = json.Unmarshal(body, &data); err != nil {
		return nil, errors.New("error unmarshalling response body")
	}

	records := gjson.Get(string(body), "records")
	if !records.Exists() {
		return nil, errors.New("no records found in response")
	}

	done := gjson.Get(string(body), "done").Bool()
	if !done {
		return nil, errors.New("request is not done")
	}

	var result []RecordIdName

	//names := make([]string, 0)
	for _, record := range records.Array() {
		name := record.Get("Name").String()
		id := record.Get("Id").String()
		result = append(result, RecordIdName{Id: id, Name: name})
	}

	return result, nil
}
