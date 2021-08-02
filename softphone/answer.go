package softphone

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/pion/interceptor"
	"github.com/pion/webrtc/v3"
)

// Answer answer an incoming call
func (s *Softphone) Answer(inviteMessage SipMessage) {

	var responseMessage SipMessage

	responseMessage = SipMessage{
		Subject: "SIP/2.0 100 Trying",
		Headers: map[string]string{
			"Via":       inviteMessage.Headers["Via"],
			"From":      inviteMessage.Headers["From"],
			"CSeq":      inviteMessage.Headers["CSeq"],
			"Call-ID":   inviteMessage.Headers["Call-ID"],
			"To":        inviteMessage.Headers["To"],
			"Supported": "outbound",
		},
		Body: "",
	}
	s.Send(responseMessage, nil)

	responseMessage = SipMessage{
		Subject: "SIP/2.0 180 Ringing",
		Headers: map[string]string{
			"Contact":   fmt.Sprintf("<sip:%s;transport=%s>", fakeEmail, strings.ToLower(s.options.Transport)),
			"Via":       inviteMessage.Headers["Via"],
			"From":      inviteMessage.Headers["From"],
			"CSeq":      inviteMessage.Headers["CSeq"],
			"Call-ID":   inviteMessage.Headers["Call-ID"],
			"To":        fmt.Sprintf("%s;tag=%s", inviteMessage.Headers["To"], uuid.New().String()),
			"Supported": "outbound",
		},
		Body: "",
	}
	s.Send(responseMessage, nil)

	mediaEngine := webrtc.MediaEngine{}
	if err := mediaEngine.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus, ClockRate: 48000},
		PayloadType:        111,
	}, webrtc.RTPCodecTypeAudio); err != nil {
		panic(err)
	}

	i := &interceptor.Registry{}
	if err := webrtc.RegisterDefaultInterceptors(&mediaEngine, i); err != nil {
		panic(err)
	}

	api := webrtc.NewAPI(webrtc.WithMediaEngine(&mediaEngine), webrtc.WithInterceptorRegistry(i))
	config := webrtc.Configuration{
		ICEServers:   []webrtc.ICEServer{},
		Certificates: []webrtc.Certificate{s.cert},
	}
	peerConnection, err := api.NewPeerConnection(config)
	if err != nil {
		panic(err)
	}

	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		fmt.Printf(">>> OnICEConnectionStateChange: %s <<<\n", connectionState.String())
	})

	peerConnection.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		if s.OnTrack != nil {
			s.OnTrack(track, nil)
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

	if _, err = peerConnection.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio); err != nil {
		panic(err)
	}

	offer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  patchFreeSwitchSDP(inviteMessage.Body),
	}
	err = peerConnection.SetRemoteDescription(offer)
	if err != nil {
		panic(err)
	}

	gatherComplete := webrtc.GatheringCompletePromise(peerConnection)

	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		panic(err)
	}
	err = peerConnection.SetLocalDescription(answer)
	if err != nil {
		panic(err)
	}

	<-gatherComplete

	responseMessage = SipMessage{
		Subject: "SIP/2.0 200 OK",
		Headers: map[string]string{
			"Contact":      fmt.Sprintf("<sip:%s;transport=%s>", fakeEmail, strings.ToLower(s.options.Transport)),
			"Content-Type": "application/sdp",
			"Via":          inviteMessage.Headers["Via"],
			"From":         inviteMessage.Headers["From"],
			"CSeq":         inviteMessage.Headers["CSeq"],
			"Call-ID":      inviteMessage.Headers["Call-ID"],
			"To":           fmt.Sprintf("%s;tag=%s", inviteMessage.Headers["To"], uuid.New().String()),
			"Allow":        "ACK,BYE,CANCEL,INFO,INVITE,MESSAGE,NOTIFY,OPTIONS,PRACK,REFER,REGISTER,SUBSCRIBE",
		},
		Body: peerConnection.LocalDescription().SDP,
	}

	s.Send(responseMessage, nil)
}
