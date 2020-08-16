package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/AvraamMavridis/randomcolor"
	"github.com/ashwanthkumar/slack-go-webhook"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

type config struct {
	RepositoryOwner string `required:"true" envconfig:"REPOSITORY_OWNER"`
	RepositoryName  string `required:"true" envconfig:"REPOSITORY_NAME"`
	GithubToken     string `required:"true" envconfig:"GITHUB_TOKEN"`
	GithubLabels    string `required:"true" envconfig:"GITHUB_LABELS"`
	MessageTitle    string `required:"true" envconfig:"MESSAGE_TITLE"`
	SlackURL        string `required:"true" envconfig:"SLACK_WEBHOOK_URL"`
}

func init() {
	log.SetFlags(log.Ldate | log.Lmicroseconds | log.Lshortfile)
}

func main() {
	if os.Getenv("ENV") == "local" {
		err := godotenv.Load()
		if err != nil {
			log.Fatal("Error loading .env file")
		}
		err = handler()
		if err != nil {
			log.Fatal(err)
		}
	} else {
		lambda.Start(handler)
	}
}

func handler() error {
	var c config
	err := envconfig.Process("", &c)
	if err != nil {
		log.Fatal(err.Error())
	}

	issues := make([]githubIssueResponse, 0)
	labelsList := strings.Split(c.GithubLabels, ":")
	wg := &sync.WaitGroup{}
	for _, l := range labelsList {
		wg.Add(1)
		tmpLabel := l
		go func() {
			defer wg.Done()
			i, err := getIssueByLabels(c.RepositoryOwner, c.RepositoryName, c.GithubToken, tmpLabel)
			if err != nil {
				log.Fatal(err)
			}
			issues = append(issues, i...)
		}()
	}

	wg.Wait()

	if len(issues) != 0 {
		err := postSlackMessage(issues, c.MessageTitle, c.SlackURL)
		if err != nil {
			log.Fatal(err)
		}
	}

	return nil
}

func getIssueByLabels(repoOwner, repoName, githubToken string, labelsName string) ([]githubIssueResponse, error) {
	log.Println("getIssueByLabels:", labelsName)
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues", repoOwner, repoName)

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", fmt.Sprintf("token %s", githubToken))

	params := req.URL.Query()
	params.Add("labels", labelsName)
	req.URL.RawQuery = params.Encode()

	client := new(http.Client)
	resp, err := client.Do(req)
	if err != nil {
		return []githubIssueResponse{}, err
	}

	byteArray, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return []githubIssueResponse{}, err
	}

	var issues []githubIssueResponse
	err = json.Unmarshal([]byte(byteArray), &issues)
	if err != nil {
		return []githubIssueResponse{}, err
	}

	return issues, nil
}

func postSlackMessage(issues []githubIssueResponse, title, slackURL string) error {
	var attachments []slack.Attachment
	for _, item := range issues {
		tmp := item
		var color string = randomcolor.GetRandomColorInHex()

		// Handle only PR
		if tmp.PullRequest.URL == "" {
			continue
		}

		updatedAt := tmp.UpdatedAt.Unix()
		attachment := slack.Attachment{
			Color:      &color,
			AuthorName: &tmp.Title,
			AuthorLink: &tmp.PullRequest.HTMLURL,
			Footer:     &tmp.User.Login,
			FooterIcon: &tmp.User.AvatarURL,
			Timestamp:  &updatedAt,
		}

		var labels []string
		for _, label := range tmp.Labels {
			labels = append(labels, label.Name)
		}
		attachment.AddField(slack.Field{Title: "Labels", Value: strings.Join(labels, ", "), Short: true})
		attachments = append(attachments, attachment)
	}
	payload := slack.Payload{
		Text:        title,
		Attachments: attachments,
	}
	err := slack.Send(slackURL, "", payload)
	if len(err) > 0 {
		fmt.Printf("error: %s\n", err)
		return err[0]
	}

	return nil
}
