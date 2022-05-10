package googlecloudloggingexporter

import (
	"io/ioutil"
	"net/http"
)

func readProjectIdMetadata() (string, error) {
	// FIXME split the URI up into variables
	req, _ := http.NewRequest("GET", "http://metadata.google.internal/computeMetadata/v1/project/project-id", nil)
	req.Header.Add("Metadata-Flavor", "Google")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	projectId := string(body)
	return projectId, nil
}
