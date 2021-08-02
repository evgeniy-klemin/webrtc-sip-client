package softphone

import (
	"log"

	"github.com/google/uuid"
	"github.com/pion/sdp/v2"
)

// Send send message via WebSocket
func (s *Softphone) Send(sipMessage SipMessage, responseHandler func(string) bool) {
	stringMessage := sipMessage.ToString()
	if s.options.Verbose {
		log.Println("↑↑↑\n", stringMessage)
	}
	if responseHandler != nil {
		var key string
		key = s.addMessageListener(func(message string) {
			done := responseHandler(message)
			if done {
				s.removeMessageListener(key)
			}
		})
	}
	err := s.conn.WriteMessage(1, []byte(stringMessage))
	if err != nil {
		log.Println(err)
		return
	}
}

func (s *Softphone) addMessageListener(messageListener func(string)) string {
	key := uuid.New().String()
	s.messageListeners[key] = messageListener
	return key
}

func (s *Softphone) removeMessageListener(key string) {
	delete(s.messageListeners, key)
}

// patchFreeSwitchSDP mid and sendrecv required for pion
func patchFreeSwitchSDP(in string) string {
	parsed := &sdp.SessionDescription{}
	if err := parsed.Unmarshal([]byte(in)); err != nil {
		log.Println(err)
		return in
	}
	for _, media := range parsed.MediaDescriptions {
		foundMid := false
		foundSendRecv := false
		for _, attr := range media.Attributes {
			switch attr.Key {
			case "mid":
				foundMid = true
			case "sendrecv":
				foundSendRecv = true
			}
		}

		// mid:0
		if !foundMid {
			media.Attributes = append(media.Attributes, sdp.NewAttribute("mid", "0"))
		}

		// sendrecv
		if !foundSendRecv {
			media.Attributes = append(media.Attributes, sdp.NewPropertyAttribute("sendrecv"))
		}
	}

	out, err := parsed.Marshal()
	if err != nil {
		panic(err)
	}
	return string(out)
}
