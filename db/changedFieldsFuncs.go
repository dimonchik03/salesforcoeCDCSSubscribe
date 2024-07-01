package db

import (
	"SalesforceGit/cdcSubscribe/common"
	"fmt"
	"strconv"
	"strings"
)

func getChangedFieldsNames(topic string, changedFieldsMap map[string][]string) map[string][]string {
	changedFieldsNames := make(map[string][]string)

	for key, value := range changedFieldsMap {
		if value[0] == "" {
			continue
		}
		//var indexOfSubElement = ""
		for _, el := range value {
			fmt.Println(el)
			if strings.Contains(el, "-") {
				parts := strings.Split(el, "-")
				fmt.Println("parts")
				fmt.Println(parts)
				fieldIndex, _ := strconv.Atoi(parts[0])
				field := common.TestSchemas[topic][fieldIndex]
				fieldName := field.Name
				var subFieldName string
				for i := 0; i < len(parts[1]); i++ {
					if string(parts[1][i]) == "1" {
						fmt.Println(field.Doc.Fields)
						subFieldName = field.Doc.Fields[i]
					}
				}
				changedFieldsNames[key] = append(changedFieldsNames[key], fieldName+"."+subFieldName)
			} else {
				for i := 0; i < len(el); i++ {
					if string(el[i]) == "1" {
						changedFieldsNames[key] = append(changedFieldsNames[key], common.TestSchemas[topic][i].Name)
					}
				}
			}
		}
	}
	return changedFieldsNames
}

func hexToReversedBinary(changedFieldsMap map[string][]string) map[string][]string {
	for _, value := range changedFieldsMap {
		if value[0] == "" {
			continue
		}
		for i, el := range value {
			if strings.Contains(el, "-") {
				parts := strings.Split(el, "-")
				hexInt, err := strconv.ParseInt(parts[1], 16, 64)
				if err != nil {
					fmt.Println("Error parsing hexadecimal:", err)
					return nil
				}
				value[i] = parts[0] + "-" + reverseString(strconv.FormatInt(hexInt, 2))
			} else {
				hexInt, err := strconv.ParseInt(el, 16, 64)
				if err != nil {
					fmt.Println("Error parsing hexadecimal:", err)
					return nil
				}
				value[i] = reverseString(strconv.FormatInt(hexInt, 2))
			}
		}
	}
	return changedFieldsMap
}

func reverseString(binStr string) string {
	runes := []rune(binStr)
	n := len(runes)
	for i := 0; i < n/2; i++ {
		runes[i], runes[n-i-1] = runes[n-i-1], runes[i]
	}
	reversedString := string(runes)

	return reversedString
}

func convertValuesToHex(changedFieldsMap map[string][]string) map[string][]string {
	for key, value := range changedFieldsMap {
		var updatedValue []string
		for _, el := range value {
			if strings.Contains(el, "-") {
				parts := strings.Split(el, "-")
				parts[1] = strings.TrimPrefix(parts[1], "0x")
				updatedValue = append(updatedValue, parts[0]+"-"+parts[1])
				//updatedValue = append(updatedValue, parts[1])
			} else {
				updatedValue = append(updatedValue, strings.TrimPrefix(el, "0x"))
			}
		}
		changedFieldsMap[key] = updatedValue
	}
	return changedFieldsMap
}
