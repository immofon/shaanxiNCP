package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"

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

type Link struct {
	Date  string
	Href  string // absolute url, which means,you can use it to access website directly.
	Title string
}

func GetLinks(pageNum int) []Link {
	page := HTTPGet(fmt.Sprintf("http://sxwjw.shaanxi.gov.cn/col/col9/index.html?uid=572&pageNum=%d", pageNum))
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

type Page struct {
	URL        string
	Title      string
	Source     string
	Date       string
	RawContent string
	Patients   []Patient
}

func GetPage(url string) Page {
	raw := HTTPGet(url)
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(raw))
	if err != nil {
		panic(err)
	}

	title := doc.Find(`#c > tbody > tr:nth-child(1) > td`).Text()
	source := doc.Find(`#c > tbody > tr:nth-child(2) > td > table > tbody > tr > td:nth-child(1)`).Text()
	source = strings.Replace(source, "来源：", "", -1)
	re := regexp.MustCompile(`.*([0-9]{4}-[0-9]{2}-[0-9]{2}).*`)
	rawDate := doc.Find(`#c > tbody > tr:nth-child(2) > td > table > tbody > tr > td:nth-child(2)`).Text()
	date := re.FindStringSubmatch(rawDate)[1] // e.g. 2020-01-23
	contentBuf := bytes.NewBuffer(nil)
	patients := make([]Patient, 0)
	doc.Find(`#zoom > p`).Each(func(i int, sel *goquery.Selection) {
		contentBuf.WriteString(sel.Text())
		contentBuf.WriteString("\n")

		text := sel.Text()
		if !strings.HasPrefix(text, "患者") {
			return
		}
		// 患者4，男， 63岁，现居西安市新城区，暂未发现明确暴露史。1月31出现症状，2月1日到唐城医院就诊。2月7日被诊断为新型冠状病毒感染的肺炎。目前在西安交通大学第二附属医院隔离治疗，病情平稳。

		re := regexp.MustCompile(`^患者([0-9]+)，(?:\s*)([^，]+)，(?:\s*)([0-9]+)岁，现居([^，。]+)(?:.*)目前([^，。]+)`)
		submatch := re.FindStringSubmatch(text)
		if len(submatch) != 6 {
			patients = append(patients, Patient{
				Detial: text,
			})
			return
		}

		id := submatch[1]
		gender := GenderUnknow
		switch submatch[2] {
		case "男":
			gender = GenderMale
		case "女":
			gender = GenderFemale
		}
		age, _ := strconv.Atoi(submatch[3])
		liveAddress := submatch[4]
		treatment := submatch[5]

		fmt.Println(submatch)
		patients = append(patients, Patient{
			Detial: text,

			ID:          fmt.Sprintf("%s:%s", date, id),
			Gender:      gender,
			Age:         age,
			LiveAddress: liveAddress,
			Treatment:   treatment,
		})
	})

	return Page{
		URL:        url,
		Title:      title,
		Source:     source,
		Date:       date,
		RawContent: contentBuf.String(),
		Patients:   patients,
	}
}

func Try(fn func()) (ok bool) {
	defer func() {
		r := recover()
		ok = r == nil
	}()

	fn()
	return true
}

func main() {
	PrintJSON(GetPage("http://sxwjw.shaanxi.gov.cn/art/2020/2/7/art_9_67829.html"))
}

func PrintJSON(v interface{}) {
	data, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(data))
}
