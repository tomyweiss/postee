package alertmgr

import (
	"github.com/aquasecurity/postee/dbservice"
	"github.com/aquasecurity/postee/eventservice"
	"github.com/aquasecurity/postee/plugins"
	"github.com/aquasecurity/postee/scanservice"
	"io/ioutil"
	"os"
	"sync"
	"testing"
	"time"
)

func TestLoads(t *testing.T) {
	cfgData := `
---
- type: common
  Max_DB_Size: 10
  Delete_Old_Data: 10
  AquaServer: https://demolab.aquasec.com
- name: jira
  type: jira
  enable: true
  url: "http://localhost:2990/jira"
  user: admin
  password: admin
  tls_verify: false
  project_key: key
  description:
  summary:
  issuetype: "Bug"
  priority: Medium
  assignee: 
  Policy-Min-Vulnerability: Critical
  labels: ["label1", "label2"]
  Policy-Min-Vulnerability: high

- name: jiraWithoutPass
  type: jira
  enable: true
  url: "http://localhost:2990/jira"
  user: admin

- name: my-slack
  type: slack
  enable: true
  url: "https://hooks.slack.com/services/TT/BBB/WWWW"

- name: email
  type: email
  enable: true
  user: EMAILUSER
  password: EMAILPASS
  host: smtp.gmail.com
  port: 587
  recipients: ["demo@gmail.com"]

- name: localEmail
  type: email
  enable: true
  useMX: true
  sender: mail@alm.demo.co
  recipients: ["demo@gmail.com"]

- name: email-empty
  type: email
  enable: true

- name: email-empty-pass
  type: email
  enable: true
  user: EMAILUSER

- name: ms-team
  type: teams
  enable: true
  url: https://outlook.office.com/webhook/.... # Webhook's url

- name: failed
  enable: true
  type: nextplugin

- name: my-servicenow
  type: serviceNow
  enable: true
  user: SERVICENOWUSER
  password: SERVICENOWPASS
  instance: dev00000

- name: noname
  type: future-plugin
  enable: true
  user: user
  password: password

- name: webhook
  type: webhook
  enable: true
  url: https://postman-echo.com/post

- name: splunk
  type: splunk
  enable: true
  url: http://localhost:8088
  token: splunk-demo-token
  SizeLimit: 20000
`
	cfgName := "cfg_test.yaml"
	ioutil.WriteFile(cfgName, []byte(cfgData), 0644)
	dbPathReal := dbservice.DbPath
	savedBaseForTicker := baseForTicker
	defer func() {
		baseForTicker = savedBaseForTicker
		os.Remove(cfgName)
		os.Remove(dbservice.DbPath)
		dbservice.ChangeDbPath(dbPathReal)
	}()
	dbservice.DbPath = "test_webhooks.db"
	baseForTicker = time.Microsecond

	demoCtx := Instance()
	demoCtx.Start(cfgName)
	pluginsNumber := 10
	if len(demoCtx.plugins) != pluginsNumber {
		t.Errorf("There are stopped plugins\nWaited: %d\nResult: %d", pluginsNumber, len(demoCtx.plugins))
	}

	_, ok := demoCtx.plugins["ms-team"]
	if !ok {
		t.Errorf("'ms-team' plugin didn't start!")
	}

	aquaWaiting := "https://demolab.aquasec.com/#/images/"
	if aquaServer != aquaWaiting {
		t.Errorf("Wrong init of AquaServer link.\nWait: %q\nGot: %q", aquaWaiting, aquaServer)
	}

	if _, ok := demoCtx.plugins["my-servicenow"]; !ok {
		t.Errorf("Plugin 'my-servicenow' didn't run!")
	}
	demoCtx.Terminate()
	time.Sleep(200 * time.Millisecond)
}

func TestServiceGetters(t *testing.T) {
	scanner := getScanService()
	if _, ok := scanner.(*scanservice.ScanService); !ok {
		t.Error("getScanService() doesn't return an instance of scanservice.ScanService")
	}
	events := getEventService()
	if _, ok := events.(*eventservice.EventService); !ok {
		t.Error("getEventService() doesn't return an instance of eventservice.EventService")
	}
}

type demoService struct {
	buff chan string
}

func (demo *demoService) ResultHandling(input string, plugins map[string]plugins.Plugin) {
	demo.buff <- input
}
func getDemoService() *demoService {
	return &demoService{
		buff: make(chan string),
	}
}

func TestSendingMessages(t *testing.T) {
	const (
		testData = "test data"
	)

	getEventServiceSaved := getEventService
	getScanServiceSaved := getScanService
	defer func() {
		getEventService = getEventServiceSaved
		getScanService = getScanServiceSaved
	}()
	dmsScan := getDemoService()
	getScanService = func() service {
		return dmsScan
	}
	dmsEvents := getDemoService()
	getEventService = func() service {
		return dmsEvents
	}
	srv := &AlertMgr{
		mutexScan:  sync.Mutex{},
		mutexEvent: sync.Mutex{},
		quit:       make(chan struct{}),
		events:     make(chan string, 1000),
		queue:      make(chan string, 1000),
		plugins:    make(map[string]plugins.Plugin),
	}
	go srv.listen()
	srv.Send(testData)
	if s := <-dmsScan.buff; s != testData {
		t.Errorf("srv.Send(%q) == %q, wanted %q", testData, s, testData)
	}
	srv.Event(testData)
	if s := <-dmsEvents.buff; s != testData {
		t.Errorf("srv.Event(%q) == %q, wanted %q", testData, s, testData)
	}
}
