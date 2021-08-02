package softphone

import (
	"fmt"
	"log"
	"strings"

	glog "github.com/ghettovoice/gosip/log"
	"github.com/ghettovoice/gosip/sip"
	"github.com/ghettovoice/gosip/sip/parser"
	"github.com/google/uuid"
	"github.com/pion/interceptor"
	"github.com/pion/webrtc/v3"
)

func (s *Softphone) Invite(extension string) {

	mediaEngine := webrtc.MediaEngine{}
	if err := mediaEngine.RegisterDefaultCodecs(); err != nil {
		panic(err)
	}
	if err := mediaEngine.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus, ClockRate: 48000},
		PayloadType:        111,
	}, webrtc.RTPCodecTypeAudio); err != nil {
		panic(err)
	}

	settingEngine := webrtc.SettingEngine{}

	i := &interceptor.Registry{}
	if err := webrtc.RegisterDefaultInterceptors(&mediaEngine, i); err != nil {
		panic(err)
	}

	api := webrtc.NewAPI(webrtc.WithMediaEngine(&mediaEngine), webrtc.WithSettingEngine(settingEngine), webrtc.WithInterceptorRegistry(i))
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
		Certificates: []webrtc.Certificate{s.cert},
	}
	peerConnection, err := api.NewPeerConnection(config)
	if err != nil {
		panic(err)
	}

	// Create a audio track
	audioTrack, err := webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus, ClockRate: 8000}, "audio", "go")
	if err != nil {
		panic(err)
	}
	_, err = peerConnection.AddTrack(audioTrack)
	if err != nil {
		panic(err)
	}

	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		fmt.Printf(">>> OnICEConnectionStateChange: %s <<<\n", connectionState.String())
	})

	peerConnection.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {

		if s.OnTrack != nil {
			s.OnTrack(track, audioTrack)
		}
	})

	peerConnection.OnICEGatheringStateChange(func(state webrtc.ICEGathererState) {
		fmt.Printf(">>> OnICEGatheringStateChange: %s <<<\n", state)
	})

	peerConnection.OnConnectionStateChange(func(conn webrtc.PeerConnectionState) {
		fmt.Printf(">>> OnConnectionStateChange: %s <<<\n", conn)
	})

	peerConnection.OnSignalingStateChange(func(sign webrtc.SignalingState) {
		fmt.Printf(">>> OnSignalingStateChange: %s <<<\n", sign)
	})

	offer, err := peerConnection.CreateOffer(nil)
	if err != nil {
		log.Panic(err)
	}

	gatherComplete := webrtc.GatheringCompletePromise(peerConnection)
	err = peerConnection.SetLocalDescription(offer)
	if err != nil {
		panic(err)
	}
	<-gatherComplete

	requestMessage := SipMessage{
		Subject: fmt.Sprintf("INVITE sip:%s SIP/2.0", s.options.Domain),
		Headers: map[string]string{
			"Contact":      fmt.Sprintf("<sip:%s;transport=%s>;expires=200", fakeEmail, strings.ToLower(s.options.Transport)),
			"To":           fmt.Sprintf("<sip:%s@%s>", extension, s.options.Domain),
			"Via":          fmt.Sprintf("SIP/2.0/%s %s;branch=z9hG4bK%s", strings.ToUpper(s.options.Transport), fakeDomain, uuid.New().String()),
			"From":         fmt.Sprintf("<sip:%s@%s>;tag=%s", s.options.Username, s.options.Domain, s.fromTag),
			"Call-ID":      s.callID,
			"Supported":    "replaces, outbound,ice",
			"Content-Type": "application/sdp",
			"CSeq":         "8083 INVITE",
			"Max-Forwards": "70",
		},
		Body: peerConnection.LocalDescription().SDP,
	}
	s.Send(requestMessage, func(strMessage string) bool {
		logger := glog.NewDefaultLogrusLogger()
		msg, err := parser.ParseMessage([]byte(strMessage), logger)
		if err != nil {
			panic(err)
		}

		if strings.Contains(msg.StartLine(), "SIP/2.0 407 Proxy Authentication Required") {
			headers := msg.GetHeaders("Proxy-Authenticate")
			auth := sip.AuthFromValue(headers[0].Value()).
				SetMethod("INVITE").
				SetUsername(s.options.Username).
				SetPassword(s.options.Password)
			auth.SetResponse(auth.CalcResponse())

			requestAuthMessage := SipMessage{
				Subject: fmt.Sprintf("INVITE sip:%s SIP/2.0", s.options.Domain),
				Headers: map[string]string{
					"Contact":      requestMessage.Headers["Contact"],
					"To":           requestMessage.Headers["To"],
					"Via":          requestMessage.Headers["Via"],
					"From":         requestMessage.Headers["From"],
					"Call-ID":      requestMessage.Headers["Call-ID"],
					"Supported":    requestMessage.Headers["Supported"],
					"Content-Type": requestMessage.Headers["Content-Type"],
					"CSeq":         requestMessage.Headers["CSeq"],
					"Max-Forwards": requestMessage.Headers["Max-Forwards"],
				},
				Body: peerConnection.LocalDescription().SDP,
			}

			requestAuthMessage.Headers["Proxy-Authorization"] = auth.String()
			requestAuthMessage.IncreaseSeq()
			s.Send(requestAuthMessage, func(strMessage string) bool {

				logger := glog.NewDefaultLogrusLogger()
				msg, err = parser.ParseMessage([]byte(strMessage), logger)
				if err != nil {
					panic(err)
				}

				if !strings.Contains(msg.StartLine(), "200 OK") {
					return false
				}

				rsd := webrtc.SessionDescription{Type: webrtc.SDPTypeAnswer, SDP: patchFreeSwitchSDP(msg.Body())}
				if err := peerConnection.SetRemoteDescription(rsd); err != nil {
					panic(err)
				}

				cseq, _ := msg.CSeq()
				cseq.MethodName = "ACK"

				responseMessage := SipMessage{
					Subject: fmt.Sprintf("ACK sip:%s@%s SIP/2.0", s.options.Username, s.options.Domain),
					Headers: map[string]string{
						"Contact":      fmt.Sprintf("<sip:%s;transport=%s>;expires=200", fakeEmail, strings.ToLower(s.options.Transport)),
						"To":           msg.GetHeaders("To")[0].Value(),
						"Via":          fmt.Sprintf("SIP/2.0/%s %s;branch=z9hG4bK%s", strings.ToUpper(s.options.Transport), fakeDomain, uuid.New().String()),
						"From":         msg.GetHeaders("From")[0].Value(),
						"Call-ID":      s.callID,
						"CSeq":         cseq.Value(),
						"Max-Forwards": "70",
					},
					Body: "",
				}
				s.Send(responseMessage, nil)

				return true

			})

			return true
		}

		if !strings.Contains(msg.StartLine(), "200 OK") {
			return false
		}

		rsd := webrtc.SessionDescription{Type: webrtc.SDPTypeAnswer, SDP: patchFreeSwitchSDP(msg.Body())}
		if err := peerConnection.SetRemoteDescription(rsd); err != nil {
			panic(err)
		}

		cseq, _ := msg.CSeq()
		cseq.MethodName = "ACK"

		responseMessage := SipMessage{
			Subject: fmt.Sprintf("ACK sip:%s@%s SIP/2.0", s.options.Username, s.options.Domain),
			Headers: map[string]string{
				"Contact":      fmt.Sprintf("<sip:%s;transport=%s>;expires=200", fakeEmail, strings.ToLower(s.options.Transport)),
				"To":           msg.GetHeaders("To")[0].Value(),
				"Via":          fmt.Sprintf("SIP/2.0/%s %s;branch=z9hG4bK%s", strings.ToUpper(s.options.Transport), fakeDomain, uuid.New().String()),
				"From":         msg.GetHeaders("From")[0].Value(),
				"Call-ID":      s.callID,
				"CSeq":         cseq.Value(),
				"Max-Forwards": "70",
			},
			Body: "",
		}
		s.Send(responseMessage, nil)

		return true
	})
}
