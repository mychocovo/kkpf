package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/gorilla/mux"
)

type ResKeyboard struct {
	Type string `json:"type"`
}

func apiKeyboardHandler(w http.ResponseWriter, r *http.Request) {
	err := json.NewEncoder(w).Encode(ResKeyboard{
		Type: "text",
	})
	if err != nil {
		log.Println(err)
	}
}

type Message struct {
	Text string `json:"text"`
}
type ResMessage struct {
	Message Message `json:"message"`
}
type ReqMessage struct {
	UserKey string
	Type    string
	Content string
}
var konum = map[rune]int{
	'일': 1, '이': 2, '삼': 3, '사': 4, '오': 5,
	'육': 6, '칠': 7, '팔': 8, '구': 9, '십': 10,
}

var replaceMap = map[string]string{
	"승": "s", "ㅅ": "s",
	"무": "d", "ㅁ": "d", "ㅇ": "d",
	"패": "f", "ㅍ": "f", "ㅂ": "f",
}

var sdfMap = map[string]string{"s": "승", "d": "무", "f": "패"}
var miscrx = regexp.MustCompile(`,|\.|ㅈ|\s`)
var pprx = regexp.MustCompile(`ㅅ|ㅁ|ㅇ|ㅍ|ㅂ|승|무|패`)
var nsdfrx = regexp.MustCompile(`s|d|f`)
var sdfrx = regexp.MustCompile(`\d+(s|d|f)`)
var monrx = regexp.MustCompile(`(\d{1,}000원{0,})|((.*만)(.*천){0,}원{0,})|(.*천)원{0,}|(\d{1,})`)

type ConQ struct {
	Q *[]string
	L *sync.RWMutex
}

func (q ConQ) Len() int {
	q.L.RLock()
	defer q.L.RUnlock()
	return len((*q.Q))
}
func (q ConQ) Head() string {
	q.L.RLock()
	defer q.L.RUnlock()
	if len((*q.Q)) > 0 {
		return (*q.Q)[0]
	} else {
		return ""
	}
}
func (q ConQ) Del() {
	q.L.Lock()
	if len((*q.Q)) > 0 {
		(*q.Q) = (*q.Q)[1:]
	}
	q.L.Unlock()
}
func (q ConQ) Add(s string) {
	q.L.Lock()
	(*q.Q) = append((*q.Q), s)
	q.L.Unlock()
}

var ss = make([]string, 0)
var cq ConQ

func process(in string) (string, bool) {
	johab := strings.Index(in, "ㅈ")
	message := ""
	state := ""
	right := true
	aa := pprx.ReplaceAllStringFunc(miscrx.ReplaceAllString(in, ""), func(expr string) string {
		return string(replaceMap[expr])
	})

	as := sdfrx.FindAllString(aa, -1)
	message += "예측 : "
	for _, s := range as {
		state += s
		message += s[0:len(s)-1] + "-" + sdfMap[s[len(s)-1:]] + " "
	}
	if len(state) == 0 {
		right = false
	}
	state += " "
	aa = sdfrx.ReplaceAllString(aa, "")

	money := monrx.FindString(aa)
	bet := 0
	switch {
	case strings.Contains(aa, "000"):
		idxwon :=  strings.Index(money, "원")
		i, err := strconv.Atoi(money)
		if err == nil {
			bet += i / 1000
		} else {
			i, err = strconv.Atoi(money[:idxwon])
			if err == nil {
				bet += i / 1000
			}
		}
	case strings.ContainsAny(aa, "만천"):
		idxman := strings.Index(money, "만")
		if idxman > 0 {
			i, err := strconv.Atoi(money[:idxman])
			if err == nil {
				bet += i * 10
			} else {
				for _, r := range money[:idxman] {
					bet += konum[r] * 10
				}
			}
			money = money[idxman + len("만"):]
		}

		idxchun := strings.Index(money, "천")
		if idxchun > 0 {
			i, err := strconv.Atoi(money[:idxchun])
			if err == nil {
				bet += i
			} else {
				for _, r := range money[:idxchun] {
					bet += konum[r]
				}
			}

		}
	default:
		i, err := strconv.Atoi(money)
		if err == nil {
			bet += i
		}
	}
	state += strconv.Itoa(bet) + " "
	if bet > 0 {
		message += fmt.Sprintf("\n금액 : %d원", bet*1000)
	}

	aa = monrx.ReplaceAllString(aa, "")
	aa = strings.Trim(aa, " .")
	if len(aa) > 0 {
		aa := nsdfrx.ReplaceAllStringFunc(aa, func(expr string) string {
			return string(sdfMap[expr])
		})
		state += aa
		message += fmt.Sprintf("\n이름 : %s", aa)
	} else {
		state += "N"
	}
	if johab >= 0 {
		state += " J"
		message += "\n조합 : O"
	} else {
		state += " N"
	}
	if right {
		cq.Add(state)
	}
	return message, right
}
func apiMessageHandler(w http.ResponseWriter, r *http.Request) {
	var reqMsg ReqMessage
	err := json.NewDecoder(r.Body).Decode(&reqMsg)
	if err != nil {
		log.Println(err)
	}
	sends := ""
	for i, s := range strings.Split(reqMsg.Content, "\n") {
		content, isCorrect := process(s)
		if len(s) > 0 {
			if i > 0 { sends += "\n=============================\n" }
			if !isCorrect {
				sends += "잘못된 입력입니다."
			} else {
				sends += content
			}
		}
	}
	err = json.NewEncoder(w).Encode(ResMessage{
		Message{
			Text: sends,
		},
	})
	if err != nil {
		log.Println(err)
	}
}

func apiFriendHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "SUCCESS")
}
func apiChatRoomHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "SUCCESS")
}

func main() {
	cq = ConQ{&ss, &sync.RWMutex{}}
	go func() {
		ln, err := net.Listen("tcp", ":11733")
		defer ln.Close()
		if err != nil {
			os.Exit(1)
		}
		for {
			conn, err := ln.Accept()
			defer conn.Close()
			if err != nil {
				os.Exit(1)
			}
			for {
				if cq.Len() > 0 {
					message := cq.Head()
					buffer := make([]byte, 512)
					copy(buffer, []byte(message))
					_, err := conn.Write(buffer)
					if err != nil {
						break
					}
					cq.Del()
				}
			}
		}
	}()

	r := mux.NewRouter()
	s := r.PathPrefix("/kkpf/").Subrouter()
	s.HandleFunc("/keyboard", apiKeyboardHandler).Methods("GET")
	s.HandleFunc("/message", apiMessageHandler).Methods("POST")
	s.HandleFunc("/friend", apiFriendHandler).Methods("POST", "DELETE")
	s.HandleFunc("/chat_room/{user_key}", apiChatRoomHandler).Methods("DELETE")
	http.Handle("/", r)
	log.Fatal(http.ListenAndServe(":18181", nil))
}
