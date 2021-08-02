package softphone

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"net/url"
	"strings"

	glog "github.com/ghettovoice/gosip/log"
	"github.com/ghettovoice/gosip/sip"
	"github.com/ghettovoice/gosip/sip/parser"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/pion/dtls/v2/examples/util"
	"github.com/pion/webrtc/v3"
)

type Options struct {
	Username  string
	Password  string
	Domain    string
	Transport string
	Host      string
	Path      string
	Port      uint16
	SRTPKey   string
	SRTPCert  string
	Verbose   bool
}

// Softphone softphone
type Softphone struct {
	options          Options
	messageListeners map[string]func(string)
	conn             *websocket.Conn
	OnInvite         func(inviteMessage SipMessage)
	OnTrack          func(remote *webrtc.TrackRemote, local *webrtc.TrackLocalStaticSample)
	fromTag          string
	callID           string
	cert             webrtc.Certificate
	auth             string
}

func New(options Options, cert webrtc.Certificate) *Softphone {
	res := &Softphone{
		options:          options,
		messageListeners: make(map[string]func(string)),
		cert:             cert,
	}
	return res
}

var fakeDomain = fmt.Sprintf("%s.invalid", uuid.New().String())
var fakeEmail = fmt.Sprintf("%s@%s", uuid.New().String(), fakeDomain)

func LoadCert(keyFile, certFile string) webrtc.Certificate {
	key, err := util.LoadKey(keyFile)
	if err != nil {
		log.Panic(err)
	}
	certRaw, err := util.LoadCertificate(certFile)
	if err != nil {
		log.Panic(err)
	}
	certx509, err := x509.ParseCertificate(certRaw.Certificate[0])
	if err != nil {
		log.Panic(err)
	}
	return webrtc.CertificateFromX509(key, certx509)
}

// Register register the softphone
func (s *Softphone) Register() {
	url := url.URL{
		Scheme: s.options.Transport,
		Host:   fmt.Sprintf("%s:%d", s.options.Host, s.options.Port),
		Path:   s.options.Path,
	}
	dialer := websocket.DefaultDialer
	dialer.Subprotocols = []string{"sip"}
	dialer.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	var err error
	s.conn, _, err = dialer.Dial(url.String(), nil)
	if err != nil {
		log.Println(err)
		return
	}
	go func() {
		for {
			_, bytes, err := s.conn.ReadMessage()
			if err != nil {
				log.Println(err)
				return
			}
			message := string(bytes)
			if s.options.Verbose {
				log.Println("↓↓↓\n", message)
			}
			for _, messageListener := range s.messageListeners {
				go messageListener(message)
			}
		}
	}()

	s.fromTag = uuid.New().String()
	s.callID = uuid.New().String()

	registerMessage := SipMessage{
		Subject: fmt.Sprintf("REGISTER sip:%s SIP/2.0", s.options.Domain),
		Headers: map[string]string{
			"Call-ID": s.callID,
			"Contact": fmt.Sprintf("<sip:%s;transport=%s>;expires=600", fakeEmail, s.options.Transport),
			"Via":     fmt.Sprintf("SIP/2.0/%s %s;branch=z9hG4bK%s", strings.ToUpper(s.options.Transport), fakeDomain, uuid.New().String()),
			"From":    fmt.Sprintf("<sip:%s@%s>;tag=%s", s.options.Username, s.options.Domain, s.fromTag),
			"To":      fmt.Sprintf("<sip:%s@%s>", s.options.Username, s.options.Domain),
			"CSeq":    "8082 REGISTER",
		},
		Body: "",
	}
	s.Send(registerMessage, func(strMessage string) bool {
		logger := glog.NewDefaultLogrusLogger()
		msg, err := parser.ParseMessage([]byte(strMessage), logger)
		if err != nil {
			return false
		}
		if strings.Contains(msg.StartLine(), "SIP/2.0 401 Unauthorized") {
			headers := msg.GetHeaders("WWW-Authenticate")
			auth := sip.AuthFromValue(headers[0].Value()).
				SetMethod("REGISTER").
				SetUsername(s.options.Username).
				SetPassword(s.options.Password)
			auth.SetResponse(auth.CalcResponse())

			registerMessage.Headers["Authorization"] = auth.String()
			s.auth = auth.String()
			registerMessage.IncreaseSeq()
			registerMessage.Headers["Via"] = fmt.Sprintf("SIP/2.0/%s %s;branch=z9hG4bK%s", strings.ToUpper(s.options.Transport), fakeDomain, uuid.New().String())
			s.Send(registerMessage, nil)

			return true
		}
		return false
	})

	s.addMessageListener(func(strMessage string) {
		if strings.Contains(strMessage, "INVITE sip:") {
			s.OnInvite(FromStringToSipMessage(strMessage))
		}
	})
}
