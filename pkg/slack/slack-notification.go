package slack

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"text/template"
)

type slackTemplate struct {
	Job interface{}
}

func getParsedTemplate(text string, data interface{}) (string, error) {
	//create a new template with some name
	tmpl := template.New("test")
	//parse some content and generate a template
	tmpl, err := tmpl.Parse(text)
	if err != nil {
		log.Fatal("Parse: ", err)
		return "", err
	}
	var tpl bytes.Buffer
	if err1 := tmpl.Execute(&tpl, data); err1 != nil {
		log.Fatal("Parse: ", err)
		return "", err
	}
	return tpl.String(), nil
}

// SendNotification Sends slack notification
func SendNotification(data map[string]string, job interface{}) {
	url := data["webhookUrl"]
	// fmt.Println("URL:>", url)

	parsedText, err := getParsedTemplate(data["text"], struct {
		Job interface{}
	}{
		job,
	})

	if err != nil {
		// something wrong with parsing of template
		return
	}

	var jsonStr = []byte(`{"text":"*` + data["subject"] + `*\n` + parsedText + `"}`)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonStr))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	// fmt.Println("response Status:", resp.Status)
	// fmt.Println("response Headers:", resp.Header)
	body, _ := ioutil.ReadAll(resp.Body)
	fmt.Println("successfully send slack notification. response Body:", string(body))
}
