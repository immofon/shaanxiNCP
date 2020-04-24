package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

func HTTPGet(url string) string {
	r, err := http.NewRequest("GET", url, nil)
	if err != nil {
		panic(err)
	}

	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	buf := bytes.NewBuffer(nil)
	_, err = io.Copy(buf, resp.Body)
	if err != nil {
		panic(err)
	}

	return buf.String()
}

func HTTPPost(url string) string {
	r, err := http.NewRequest("POST", url, nil)
	if err != nil {
		panic(err)
	}

	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	buf := bytes.NewBuffer(nil)
	_, err = io.Copy(buf, resp.Body)
	if err != nil {
		panic(err)
	}

	return buf.String()
}

type Link struct {
	Date  string
	Href  string // absolute url, which means,you can use it to access website directly.
	Title string
}

func GetLinks(state State, pageNum int) []Link {
	page := HTTPGet("http://sxwjw.shaanxi.gov.cn/col/col9/index.html?uid=572&pageNum=1")
	re := regexp.MustCompile(`<a href="(.+)" target="_blank">([^<]+)</a><span>([0-9]{4}-[0-9]{2}-[0-9]{2})</span></li>`)
	recards := re.FindAllStringSubmatch(page, -1)

	links := make([]Link, len(recards))
	for i, recard := range recards {
		links[i] = Link{
			Date:  recard[3],
			Href:  "http://sxwjw.shaanxi.gov.cn" + recard[1],
			Title: recard[2],
		}
	}

	return links
}

type Gender string

const (
	GenderUnknow Gender = ""
	GenderMale   Gender = "male"
	GenderFemale Gender = "female"
)

type Patient struct {
	Detial string // 原始信息

	ID          string
	Gender      Gender
	Age         int
	LiveAddress string // 居住地
	Treatment   string // 治疗地点
}

func PatientIDLessThan(id1, id2 string) bool {
	r1 := strings.Split(id1, ":")
	r2 := strings.Split(id2, ":")

	d1 := r1[0]
	d2 := r2[0]
	n1, err := strconv.Atoi(r1[1])
	if err != nil {
		panic(err)
	}
	n2, err := strconv.Atoi(r2[1])
	if err != nil {
		panic(err)
	}

	t1, err := time.Parse("2006-01-02", d1)
	if err != nil {
		panic(err)
	}
	t2, err := time.Parse("2006-01-02", d2)
	if err != nil {
		panic(err)
	}

	if t2.Sub(t1) > 0 {
		return true
	} else if t2.Sub(t1) == 0 {
		return n1 < n2
	} else {
		return false
	}
}

type Page struct {
	URL        string
	OK         bool
	Title      string
	Source     string
	Date       string
	RawContent string
	Patients   []Patient
}

func GetPage(url string, content string) (page Page) {
	defer func() {
		page.OK = recover() == false
	}()
	page.URL = url

	raw := content
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(raw))
	if err != nil {
		panic(err)
	}

	title := doc.Find(`#c > tbody > tr:nth-child(1) > td`).Text()
	page.Title = title

	source := doc.Find(`#c > tbody > tr:nth-child(2) > td > table > tbody > tr > td:nth-child(1)`).Text()
	source = strings.Replace(source, "来源：", "", -1)
	page.Source = source

	re := regexp.MustCompile(`.*([0-9]{4}-[0-9]{2}-[0-9]{2}).*`)
	rawDate := doc.Find(`#c > tbody > tr:nth-child(2) > td > table > tbody > tr > td:nth-child(2)`).Text()
	date := re.FindStringSubmatch(rawDate)[1] // e.g. 2020-01-23
	page.Date = date

	contentBuf := bytes.NewBuffer(nil)
	patients := make([]Patient, 0)
	doc.Find(`#zoom > p`).Each(func(i int, sel *goquery.Selection) {
		text := sel.Text()
		replace_map := map[string]string{
			"，": ",",
			" ": "",
		}

		for k, v := range replace_map {
			text = strings.Replace(text, k, v, -1)
		}

		contentBuf.WriteString(text)
		contentBuf.WriteString("\n")

		if !strings.HasPrefix(text, "患者") {
			return
		}

		//患者6,女, 27岁 ,现居安康市白河县,长期在武汉市工作,系2月5日确诊患者18的女儿。1月20日从武汉市到安康市白河县,2月4日被转运至白河县医院隔离,患者胸部CT显示右肺下叶感染并胸膜增厚。2月6日被诊>     断为新型冠状病毒感染的肺炎。目前在白河县医院隔离治疗,病情平稳。
		// 患者4，男， 63岁，现居西安市新城区，暂未发现明确暴露史。1月31出现症状，2月1日到唐城医院就诊。2月7日被诊断为新型冠状病毒感染的肺炎。目前在西安交通大学第二附属医院隔离治疗，病情平稳。
		re := regexp.MustCompile(`^患者([0-9]+),(?:\s*)([^,。]+),(?:\s*)([0-9]+)岁,现居([^,。]+)(?:.*)目前([^,。]+)`)
		submatch := re.FindStringSubmatch(text)
		var (
			id          string
			gender      Gender
			age         int
			liveAddress string
			treatment   string
		)
		if len(submatch) != 6 {
			// 患者3,男,22岁,西安市人。1月15至17日于武汉同济医院做牙齿正畸,17日晚返回西安。20日出现发热症状,21日在省传染病院（西安市八院）隔离治疗。
			re = regexp.MustCompile(`^患者([0-9]+),(?:\s*)([^,]+),(?:\s*)([0-9]+)岁,([^人,。]+)(?:.*)在([^(隔离治疗)]+)`)
			submatch = re.FindStringSubmatch(text)
		}
		if len(submatch) != 6 {
			// 患者9，男，27岁，咸阳市人，在武汉工作。1月18日返回咸阳市乾县，21日出现症状，当天在乾县人民医院就诊。25日被确诊为新型冠状病毒感染的肺炎。目前在乾县人民医院乾陵分院隔离治疗，病情平稳
			re = regexp.MustCompile(`^患者([0-9]+),(?:\s*)([^,]+),(?:\s*)([0-9]+)岁,([^人,。]+)(?:.*)目前([^,。]+)`)
			submatch = re.FindStringSubmatch(text)
		}

		if len(submatch) != 6 {
			patients = append(patients, Patient{
				Detial: text,
			})
			return
		}
		id = submatch[1]
		gender = GenderUnknow
		switch submatch[2] {
		case "男":
			gender = GenderMale
		case "女":
			gender = GenderFemale
		}
		age, _ = strconv.Atoi(submatch[3])
		liveAddress = submatch[4]
		treatment = submatch[5]

		patients = append(patients, Patient{
			Detial: text,

			ID:          fmt.Sprintf("%s:%s", date, id),
			Gender:      gender,
			Age:         age,
			LiveAddress: liveAddress,
			Treatment:   treatment,
		})
	})

	page.RawContent = contentBuf.String()
	page.Patients = patients
	return
}

func Try(fn func()) (ok bool) {
	defer func() {
		r := recover()
		ok = r == nil
	}()

	fn()
	return true
}

type State struct {
	Links []Link `json:"links"`

	Contents map[string]string `json:"contents"` // key: links
	Pages    map[string]Page   `json:"pages"`    // key: links
}

func (s State) HadLink(link Link) bool {
	for _, l := range s.Links {
		if l.Href == link.Href {
			return true
		}
	}
	return false
}

func (s *State) Init() {
	if s.Links == nil {
		s.Links = make([]Link, 0, 100)
	}

	if s.Contents == nil {
		s.Contents = make(map[string]string)
	}
	if s.Pages == nil {
		s.Pages = make(map[string]Page)
	}

}

func stage_get_links(state *State) {
	ret := false
	for i := 1; i < 1000; i++ {
		links := GetLinks(*state, i)
		for _, link := range links {
			if state.HadLink(link) {
				ret = true
			} else {
				state.Links = append(state.Links, link)
				PrintJSON(link)
			}
		}

		if ret {
			return
		}
	}
}

func stage_get_pages(state *State) {
	for i, link := range state.Links {
		fmt.Println("getting pages:", i+1, len(state.Links))
		page := GetPage(link.Href, state.Contents[link.Href])
		state.Pages[link.Href] = page
	}
}

func stage_get_raw(state *State) {
	for i, link := range state.Links {
		fmt.Println("getting raw:", i+1, len(state.Links))

		if _, had := state.Contents[link.Href]; had == false {
			raw := HTTPGet(link.Href)
			state.Contents[link.Href] = raw
		}
	}

}

func main() {
	var state State
	data, err := ioutil.ReadFile("./state.json")
	if err == nil {
		err = json.Unmarshal(data, &state)
		if err != nil {
			panic(err)
		}
	}

	state.Init()

	switch os.Getenv("stage") {
	case "link":
		stage_get_links(&state)
	case "raw":
		stage_get_raw(&state)
	case "page":
		stage_get_pages(&state)
	case "count_patients":
		count := 0
		hasID := 0
		for _, page := range state.Pages {
			for _, p := range page.Patients {
				count++
				if p.ID != "" {
					hasID++
				}
			}
		}
		fmt.Println("patients number:", count)
		fmt.Println("has ID:", hasID)
	case "patient":
		patients := make([]Patient, 0, 300)
		for _, page := range state.Pages {
			for _, p := range page.Patients {
				patients = append(patients, p)
			}
		}

		sort.Slice(patients, func(i, j int) bool {
			return PatientIDLessThan(patients[i].ID, patients[j].ID)
		})

		fmt.Println("#id,address,age,gender,live_address,treatment_address")
		for _, p := range patients {
			p.LiveAddress = strings.Replace(p.LiveAddress, "陕西省", "", -1)
			p.LiveAddress = strings.Replace(p.LiveAddress, "长期在", "", -1)
			p.LiveAddress = strings.Replace(p.LiveAddress, "居住", "", -1)
			addresses := strings.Split(p.LiveAddress, "市")

			address_map := map[string]string{
				"武汉": "湖北省武汉",
			}

			for k, v := range address_map {
				if addresses[0] == k {
					addresses[0] = v
					break
				}
			}

			fmt.Printf("%v,%v,%v,%v,%v,%v\n", p.ID, addresses[0], p.Age, p.Gender, p.LiveAddress, p.Treatment)
		}

		data, err := json.MarshalIndent(patients, "", "  ")
		if err != nil {
			panic(err)
		}

		ioutil.WriteFile("./patient.json", data, 0600)
	case "unprocess":
		keys := []string{"新冠", "新型冠状", "境外输入"}
		for _, page := range state.Pages {
			if len(page.Patients) > 0 {
				continue
			}
		NEXT_PAGE:
			for _, key := range keys {
				if strings.Contains(page.RawContent, key) {
					PrintJSON(page)
					break NEXT_PAGE
				}
			}
		}
	default:
		fmt.Println("env state = (link|raw|page|count_patients|patient)")
	}

	data, err = json.MarshalIndent(&state, "", "  ")
	if err != nil {
		panic(err)
	}

	err = ioutil.WriteFile("./state.json.tmp", data, 0600)
	if err != nil {
		panic(err)
	}

	os.Remove("./state.json")

	err = os.Rename("./state.json.tmp", "./state.json")
	if err != nil {
		panic(err)
	}
}

func PrintJSON(v interface{}) {
	data, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(data))
}
