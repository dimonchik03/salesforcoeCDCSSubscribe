package db

import (
	"strings"
)

func setupChangeEventHeader(topic string, changeEventHeader interface{}) map[string]interface{} {
	changeEventHeaderMap := changeEventHeader.(map[string]interface{})

	// get fields
	nulledFields := changeEventHeaderMap["nulledFields"].([]interface{})
	changedFields := changeEventHeaderMap["changedFields"].([]interface{})
	diffFields := changeEventHeaderMap["diffFields"].([]interface{})
	// convert the arrays of interface{} to arrays of strings
	nulledFieldsStr, changedFieldsStr, diffFieldsStr := convertInterfaceToString(nulledFields, changedFields, diffFields)

	changedFieldsMap := map[string][]string{
		"nulledFields":  strings.Split(strings.Join(nulledFieldsStr, ", "), ", "),
		"changedFields": strings.Split(strings.Join(changedFieldsStr, ", "), ", "),
		"diffFields":    strings.Split(strings.Join(diffFieldsStr, ", "), ", "),
	}
	changedFieldsMap = getChangedFieldsNames(topic, hexToReversedBinary(convertValuesToHex(changedFieldsMap)))
	changeEventHeaderMap["nulledFields"] = changedFieldsMap["nulledFields"]
	changeEventHeaderMap["changedFields"] = changedFieldsMap["changedFields"]
	changeEventHeaderMap["diffFields"] = changedFieldsMap["diffFields"]

	//changedFieldsMap = getFieldsNames(hexToReversedBinary(convertValuesToHex(changedFieldsMap)))

	return changeEventHeaderMap
}
