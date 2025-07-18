package main

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/oracle/oci-go-sdk/v65/common"
)

// Message 是 Telegram Bot API 响应的结构体
type Message struct {
	OK          bool `json:"ok"`
	Result      `json:"result"`
	ErrorCode   int    `json:"error_code"`
	Description string `json:"description"`
}

// Result 包含在成功的 API 响应中的具体结果
type Result struct {
	MessageId int `json:"message_id"`
}

// sendMessage 发送一条新的 Telegram 消息
func sendMessage(name, text string) (msg Message, err error) {
	if token == "" || chat_id == "" {
		return msg, nil
	}

	data := url.Values{
		"parse_mode": {"Markdown"},
		"chat_id":    {chat_id},
		"text":       {"🔰*甲骨文通知* " + name + "\n" + text},
	}

	req, err := http.NewRequest(http.MethodPost, sendMessageUrl, strings.NewReader(data.Encode()))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := common.BaseClient{HTTPClient: &http.Client{}}
	setProxyOrNot(&client)

	var resp *http.Response
	resp, err = client.HTTPClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	var body []byte
	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	err = json.Unmarshal(body, &msg)
	if err != nil {
		return
	}

	if !msg.OK {
		err = errors.New(msg.Description)
	}

	return
}

// editMessage 编辑一条已发送的 Telegram 消息
func editMessage(messageId int, name, text string) (msg Message, err error) {
	if token == "" || chat_id == "" {
		return msg, nil
	}

	data := url.Values{
		"parse_mode": {"Markdown"},
		"chat_id":    {chat_id},
		"message_id": {strconv.Itoa(messageId)},
		"text":       {"🔰*甲骨文通知* " + name + "\n" + text},
	}

	req, err := http.NewRequest(http.MethodPost, editMessageUrl, strings.NewReader(data.Encode()))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := common.BaseClient{HTTPClient: &http.Client{}}
	setProxyOrNot(&client)

	var resp *http.Response
	resp, err = client.HTTPClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	var body []byte
	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	err = json.Unmarshal(body, &msg)
	if err != nil {
		return
	}

	if !msg.OK {
		err = errors.New(msg.Description)
	}

	return
}
