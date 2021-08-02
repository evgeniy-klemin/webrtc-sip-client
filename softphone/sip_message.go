package softphone

import (
	"fmt"
	"log"
	"strconv"
	"strings"
)

// SipMessage SIP message
type SipMessage struct {
	Subject string
	Headers map[string]string
	Body    string
}

// ToString from SipMessage to string message
func (sipMessage SipMessage) ToString() (message string) {
	if _, ok := sipMessage.Headers["Content-Length"]; !ok {
		sipMessage.Headers["Content-Length"] = fmt.Sprintf("%d", len(sipMessage.Body))
	}
	sipMessage.Headers["User-Agent"] = "github.com/evgeniy-klemin/webrtc-sip-client"
	list := []string{}
	list = append(list, sipMessage.Subject)
	for key, value := range sipMessage.Headers {
		list = append(list, fmt.Sprintf("%s: %s", key, value))
	}
	list = append(list, "")
	list = append(list, sipMessage.Body)
	return strings.Join(list, "\r\n")
}

// IncreaseSeq increase CSeq
func (sipMessage *SipMessage) IncreaseSeq() {
	if value, ok := sipMessage.Headers["CSeq"]; ok {
		tokens := strings.Split(value, " ")
		i, err := strconv.Atoi(tokens[0])
		if err != nil {
			log.Fatal("CSeq doesn't start with an integer")
		}
		tokens[0] = fmt.Sprintf("%d", i+1)
		sipMessage.Headers["CSeq"] = strings.Join(tokens, " ")
	}
}

// FromStringToSipMessage from string message to SipMessage
func FromStringToSipMessage(message string) (sipMessage SipMessage) {
	paragraphs := strings.Split(message, "\r\n\r\n")
	body := strings.Join(paragraphs[1:], "\r\n\r\n")
	paragraphs = strings.Split(paragraphs[0], "\r\n")
	subject := paragraphs[0]
	headers := make(map[string]string)
	for _, line := range paragraphs[1:] {
		tokens := strings.Split(line, ": ")
		headers[tokens[0]] = tokens[1]
	}
	return SipMessage{
		Subject: subject,
		Headers: headers,
		Body:    body,
	}
}
