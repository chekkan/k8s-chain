package config

import (
	"io/ioutil"
	"log"

	yaml "gopkg.in/yaml.v2"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
)

type SnifferConfig struct {
	Triggers []struct {
		Resource string
		State    string
		Filter   struct {
			Name      string
			Namespace string
		}
		Actions []SnifferTriggerAction `yaml:"-"`
	}
}

type SnifferTriggerAction struct {
	Type string
	Data map[string]string
}

type secretKeyRefStruct struct {
	name string
	key  string
}

type valueFrom struct {
	secretKeyRef secretKeyRefStruct
}

func convertToMapOfString(m interface{}) map[string]interface{} {
	m2 := make(map[string]interface{})
	for key, value := range m.(map[interface{}]interface{}) {
		switch key := key.(type) {
		case string:
			m2[key] = value
		}
	}
	return m2
}

func (s *secretKeyRefStruct) fillStruct(m map[string]interface{}) {
	s.name = m["name"].(string)
	s.key = m["key"].(string)
}

func (s *valueFrom) fillStruct(m map[string]interface{}) {
	secretKeyRef := &secretKeyRefStruct{}
	secretKeyRef.fillStruct(convertToMapOfString(m["secretKeyRef"]))
	s.secretKeyRef = *secretKeyRef
}

func retrieveSecret(name string, key string, secrets v1core.SecretInterface) string {
	secret, err := secrets.Get(name, meta_v1.GetOptions{})
	if err != nil {
		log.Println("error retrieving secret", name)
		log.Println(err)
		return ""
	}
	if secret.Data[key] == nil {
		log.Println("cannot find secret key", key)
		return ""
	}
	return string(secret.Data[key])
}

func replaceSecrets(yamlFile []byte, conf *SnifferConfig, secrets v1core.SecretInterface) {

	var mapkv map[string]interface{}
	err := yaml.Unmarshal(yamlFile, &mapkv)
	if err != nil {
		log.Fatalf("error trying to unmarshall config to map[string]interface{}: %v", err)
	}

	for tidx, trigger := range mapkv["triggers"].([]interface{}) {
		trigger := convertToMapOfString(trigger)
		for _, action := range trigger["actions"].([]interface{}) {
			action := convertToMapOfString(action)
			data := convertToMapOfString(action["data"].(map[interface{}]interface{}))
			for key, entry := range data {
				// the value of the data entry is an object
				// i.e. key { }
				if entry, ok := entry.(map[interface{}]interface{}); ok {
					entry := convertToMapOfString(entry)
					vf := &valueFrom{}
					vf.fillStruct(convertToMapOfString(entry["valueFrom"]))
					// store the string value
					data[key] = retrieveSecret(vf.secretKeyRef.name, vf.secretKeyRef.key, secrets)
				}
			}
			action["data"] = data
			actionYaml, err := yaml.Marshal(action)
			log.Println(string(actionYaml))
			if err == nil {
				var act SnifferTriggerAction
				err = yaml.Unmarshal(actionYaml, &act)
				if err != nil {
					log.Printf("cannot unmarshal back action %s", actionYaml)
				} else {
					conf.Triggers[tidx].Actions = append(conf.Triggers[tidx].Actions, act)
				}
			} else {
				log.Printf("Could not mashal action %s", action)
			}
		}
	}
}

func (config *SnifferConfig) ParseConfiguration(secrets v1core.SecretInterface) {
	yamlFile, err := ioutil.ReadFile("config.yaml")
	if err != nil {
		log.Printf("yamlFile.Get err   #%v ", err)
	}

	err = yaml.Unmarshal(yamlFile, config)
	if err != nil {
		log.Fatalf("Unmarshal: %v", err)
	}

	replaceSecrets(yamlFile, config, secrets)
}
