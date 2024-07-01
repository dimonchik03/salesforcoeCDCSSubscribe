package db

import (
	"SalesforceGit/cdcSubscribe/common"
	"SalesforceGit/soql"
	"encoding/json"
	"fmt"
	"github.com/tidwall/gjson"
	"go.mongodb.org/mongo-driver/bson"
	"log"
	"strconv"
	"strings"
	"time"
)

type ChangeEventHeader struct {
	SequenceNumber  int64    `bson:"sequenceNumber"`
	CommitNumber    int64    `bson:"commitNumber"`
	NulledFields    []string `bson:"nulledFields"`
	DiffFields      []string `bson:"diffFields"`
	EntityName      string   `bson:"entityName"`
	RecordIds       []string `bson:"recordIds"`
	ChangeType      string   `bson:"changeType"`
	CommitUser      string   `bson:"commitUser"`
	ChangedFields   []string `bson:"changedFields"`
	ChangeOrigin    string   `bson:"changeOrigin"`
	TransactionKey  string   `bson:"transactionKey"`
	CommitTimestamp int64    `bson:"commitTimestamp"`
}

type ChangeEvent struct {
	Event       bson.D
	EventHeader ChangeEventHeader
}

type ObjectStruct struct {
	RecordId string
	UserId   string
	Date     int64
}

func GroupEventsByid(events []bson.D) map[string][]bson.D {
	groupedEvents := make(map[string][]bson.D)
	for _, doc := range events {
		changeEventHeader, _ := doc.Map()["ChangeEventHeader"].(bson.D)
		recordIdValue, _ := changeEventHeader.Map()["recordIds"].(bson.A)
		id, _ := recordIdValue[0].(string)
		groupedEvents[id] = append(groupedEvents[id], doc)
	}
	return groupedEvents
}

func ConvertBsonArrayToMap(events []bson.D) []map[string][]bson.D {
	var dataMap []map[string][]bson.D
	for _, event := range events {
		m := make(map[string][]bson.D)
		for _, item := range event {
			if item.Value != nil {
				m[item.Key] = append(m[item.Key], item.Value.(bson.D))
			} else {
				// insert empty bson.D value if value is empty
				m[item.Key] = append(m[item.Key], bson.D{})
			}
		}
		dataMap = append(dataMap, m)
	}

	return dataMap
}

func GetFieldsNames(schema string) []common.FieldsNamesFromSchema {
	var result []common.FieldsNamesFromSchema
	var data map[string]interface{}
	err := json.Unmarshal([]byte(schema), &data)
	if err != nil {
		fmt.Println(err)
		return result
	}
	fields, ok := data["fields"].([]interface{})
	if !ok {
		return result
	}
	for _, field := range fields {
		f, ok := field.(map[string]interface{})
		if !ok {
			continue
		}
		name, ok := f["name"].(string)
		if !ok {
			continue
		}
		subFields := []string{}
		if f["type"] != nil {
			switch t := f["type"].(type) {
			case map[string]interface{}:
				if t["fields"] != nil {
					for _, subField := range t["fields"].([]interface{}) {
						sf, ok := subField.(map[string]interface{})
						if !ok {
							continue
						}
						subName, ok := sf["name"].(string)
						if !ok {
							continue
						}
						subFields = append(subFields, subName)
					}
				}
			case []interface{}:
				for _, v := range t {
					switch vt := v.(type) {
					case map[string]interface{}:
						if vt["fields"] != nil {
							for _, subField := range vt["fields"].([]interface{}) {
								sf, ok := subField.(map[string]interface{})
								if !ok {
									continue
								}
								subName, ok := sf["name"].(string)
								if !ok {
									continue
								}
								subFields = append(subFields, subName)
							}
						}
					default:
						continue
					}
				}
			default:
				continue
			}
		}
		result = append(result, common.FieldsNamesFromSchema{Name: name, SubFields: subFields})
	}
	return result
}

func convertInterfaceToString(nulledFields []interface{}, changedFields []interface{}, diffFields []interface{}) ([]string, []string, []string) {
	nulledFieldsStr := make([]string, len(nulledFields))
	for i, v := range nulledFields {
		if str, ok := v.(string); ok {
			nulledFieldsStr[i] = str
		}
	}

	changedFieldsStr := make([]string, len(changedFields))
	for i, v := range changedFields {
		if str, ok := v.(string); ok {
			changedFieldsStr[i] = str
		}
	}

	diffFieldsStr := make([]string, len(diffFields))
	for i, v := range diffFields {
		if str, ok := v.(string); ok {
			diffFieldsStr[i] = str
		}
	}
	return nulledFieldsStr, changedFieldsStr, diffFieldsStr
}

func convertBSONARRIntoBSON(event map[string][]bson.D, eventHeaderStruct ChangeEventHeader) bson.D {
	var result bson.D
	var changedFields []string
	for _, changedField := range eventHeaderStruct.ChangedFields {
		changedFields = append(changedFields, changedField)
	}
	for _, nulledField := range eventHeaderStruct.NulledFields {
		changedFields = append(changedFields, nulledField)
	}
	for _, diffField := range eventHeaderStruct.DiffFields {
		changedFields = append(changedFields, diffField)
	}
	jsonBytes, err := bson.MarshalExtJSON(event, true, true)
	if err != nil {
		fmt.Println("Error:", err)
	}

	for _, changedField := range changedFields {
		if strings.Contains(changedField, ".") {
			fields := strings.Split(changedField, ".")
			expression := fields[0]
			if len(fields) > 1 {
				expression += ".#.*." + strings.Join(fields[1:], ".*")
			}
			expression += ".*"
			value := gjson.Get(string(jsonBytes), expression)

			if value.IsArray() {
				// Convert the array value to a slice of strings
				index := false
				for _, v := range value.Array() {
					if !index {
						result = append(result, bson.E{Key: changedField, Value: v.String()})
					}
					index = true
				}
			}
		} else {
			for key, field := range event {
				if key == changedField {
					for _, bsonD := range field {
						for _, bsonE := range bsonD {
							result = append(result, bson.E{Key: key, Value: bsonE.Value})
						}
					}
				}
			}

			//value := gjson.Get(string(jsonBytes), changedField+".0")
			//result = append(result, bson.E{Key: changedField, Value: value.Value()})
		}

	}
	return result
}

func SetupEventForOutput(events []map[string][]bson.D) []ChangeEvent {
	var changeEventsStruct []ChangeEvent
	for i, event := range events {
		eventHeaderBson := event["ChangeEventHeader"]

		eventHeaderStruct := unmarshalHeader(eventHeaderBson)

		delete(events[i], "ChangeEventHeader")
		bsonForEvent := convertBSONARRIntoBSON(event, eventHeaderStruct)

		preparedEvent := ChangeEvent{
			Event:       bsonForEvent,
			EventHeader: eventHeaderStruct,
		}
		changeEventsStruct = append(changeEventsStruct, preparedEvent)
	}
	var userIds []string
	for _, event := range changeEventsStruct {
		userIds = append(userIds, event.EventHeader.CommitUser)
	}
	userNames, err := soql.GetNamesById("User", userIds)
	if err != nil {
		log.Println(err)
	}
	for i, event := range changeEventsStruct {
		for _, values := range userNames {
			if event.EventHeader.CommitUser == values.Id {
				changeEventsStruct[i].EventHeader.CommitUser = values.Name
			}
		}
	}

	return changeEventsStruct
}

func SetupObjects(objects []bson.D) []ObjectStruct {
	var result []ObjectStruct
	for _, objects := range objects {
		var obj ObjectStruct
		for _, field := range objects {
			if field.Key == "recordIds" {
				if recordIds, ok := field.Value.(bson.A); ok && len(recordIds) > 0 {
					obj.RecordId = recordIds[0].(string)
				}
			} else if field.Key == "commitUser" {
				obj.UserId = field.Value.(string)
			} else if field.Key == "lastModifiedDate" {
				if dateDoc, ok := field.Value.(bson.D); ok && len(dateDoc) > 0 {
					if dateVal, ok := dateDoc[0].Value.(float64); ok {
						obj.Date = int64(dateVal)
					} else if dateVal, ok := dateDoc[0].Value.(int64); ok {
						obj.Date = dateVal
					}
				}
			}
		}
		result = append(result, obj)
	}
	return result
}

func unmarshalHeader(headerBson []bson.D) ChangeEventHeader {
	var result ChangeEventHeader
	for _, header := range headerBson {
		headerBytes, err := bson.Marshal(header)
		if err != nil {
			fmt.Println("Error marshaling header:", err)
			continue
		}
		err = bson.Unmarshal(headerBytes, &result)
		if err != nil {
			fmt.Println("Error unmarshaling header:", err)
			continue
		}
	}
	return result
}

func ParseSchema(topic string, schema string) []common.Field {
	var result []common.Field
	amountOfNames := int(gjson.Get(schema, "fields.#").Num)
	for i := 0; i < amountOfNames; i++ {
		path := "fields." + strconv.FormatInt(int64(i), 10)
		typePath := path + ".type"
		namePath := path + ".name"
		docPath := path + ".doc"

		name := gjson.Get(schema, namePath).String()
		docName := gjson.Get(schema, docPath).String()
		fieldsIndex := int(gjson.Get(schema, typePath+".#").Num) - 1

		docNamePath := typePath + "." + strconv.FormatInt(int64(fieldsIndex), 10) + ".name"
		fieldsPath := typePath + "." + strconv.FormatInt(int64(fieldsIndex), 10) + ".fields"

		entityName := gjson.Get(schema, docNamePath).String()

		//fmt.Println(gjson.Get(schema, fieldsPath).Value())
		doc := getDocFromDocs(topic, docName)

		if doc.DocName == "" {
			subFieldsJson := gjson.Get(schema, fieldsPath)

			subFieldsSlice := getSubfields(subFieldsJson)
			doc = common.Doc{
				DocName: docName,
				Name:    entityName,
				Fields:  subFieldsSlice,
			}

			updateDocInDocs(topic, doc)
		}

		field := common.Field{
			Name: name,
			Doc:  doc,
		}

		result = append(result, field)
	}
	fmt.Println(result)
	return result
}

func getSubfields(subFields gjson.Result) []string {
	var subFieldsInDoc []string
	if subFields.Value() != nil {
		amountOfSubfields := int(gjson.Get(subFields.String(), "#").Num)

		for j := 0; j < amountOfSubfields; j++ {
			subField := gjson.Get(subFields.String(), strconv.FormatInt(int64(j), 10)+".name")
			subFieldsInDoc = append(subFieldsInDoc, subField.String())
		}

	}
	return subFieldsInDoc
}

func updateDocInDocs(topic string, doc common.Doc) {
	isInDocTypes := false
	for _, el := range common.DocTypes[topic] {
		if el.DocName == doc.DocName {
			isInDocTypes = true
			break
		}
	}

	if !isInDocTypes {
		common.DocTypes[topic] = append(common.DocTypes[topic], doc)
	}
}

func getDocFromDocs(topic string, docName string) common.Doc {

	res := common.Doc{}
	for _, el := range common.DocTypes[topic] {
		if el.DocName == docName {
			return el
		}
	}

	return res
}

func FormatDate(unixTimestamp int64) (string, error) {
	// Convert milliseconds to seconds
	unixSeconds := unixTimestamp / 1000
	fmt.Println("here")
	// Parse the timezone
	loc, err := time.LoadLocation(common.DateFormatValues.Timezone)
	if err != nil {
		return "", err
	}
	fmt.Println("here1")
	// Create a time.Time object
	t := time.Unix(unixSeconds, 0).In(loc)
	fmt.Println("here2")

	// Format the date based on the selected format
	var dateStr string
	fmt.Println(common.DateFormatValues)
	switch common.DateFormatValues.DateFormat {
	case "dd.mm.yyyy":
		dateStr = t.Format("02.01.2006")
	case "mm.dd.yyyy":
		dateStr = t.Format("01.02.2006")
	case "yyyy.mm.dd":
		dateStr = t.Format("2006.01.02")
	case "dd/mm/yyyy":
		dateStr = t.Format("02/01/2006")
	case "mm/dd/yyyy":
		dateStr = t.Format("01/02/2006")
	case "yyyy/mm/dd":
		dateStr = t.Format("2006/01/02")
	case "dd-mm-yyyy":
		dateStr = t.Format("02-01-2006")
	case "mm-dd-yyyy":
		dateStr = t.Format("01-02-2006")
	case "yyyy-mm-dd":
		dateStr = t.Format("2006-01-02")
	default:
		return "", fmt.Errorf("unsupported date format: %s", common.DateFormatValues.DateFormat)
	}
	// Format the time based on the selected format
	var timeStr string
	switch common.DateFormatValues.TimeFormat {
	case "24hour":
		timeStr = t.Format("15:04")
	case "12hour":
		timeStr = t.Format("3:04 PM")
	case "24hourWithSeconds":
		timeStr = t.Format("15:04:05")
	case "12hourWithSeconds":
		timeStr = t.Format("3:04:05 PM")
	default:
		return "", fmt.Errorf("unsupported time format: %s", common.DateFormatValues.TimeFormat)
	}
	fmt.Println(dateStr + " " + timeStr)
	// Combine date and time
	return dateStr + " " + timeStr, nil
}
