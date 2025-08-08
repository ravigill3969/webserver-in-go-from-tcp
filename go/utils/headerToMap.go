package utils

import "strings"

func HeaderToMap(request string) map[string]string {
	headersMap := map[string]string{}
	splitRequestInfoRN := strings.Split(request, "\r\n")

	for i := 1; i < len(splitRequestInfoRN); i++ {
		currentLine := splitRequestInfoRN[i]
		if currentLine == "" {
			break
		}
		for j := 0; j < len(currentLine); j++ {
			if currentLine[j] == ':' {
				key := strings.TrimSpace(currentLine[:j])
				value := strings.TrimSpace(currentLine[j+1:])

				headersMap[key] = value
				break
			}
		}

	}

	return headersMap

}
