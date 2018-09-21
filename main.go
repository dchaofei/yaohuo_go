package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

type meatPost struct {
	urlStr,
	title string
	totalMoney,
	balance,
	assign,
	classid,
	lpage,
	id int
}

const (
	// BaseURI 网站域名
	BaseURI = "https://yaohuo.me"
	// LoginURI 登录 url
	LoginURI = BaseURI + "/waplogin.aspx"
	// ReplyURI 回复 url
	ReplyURI = "https://yaohuo.me/bbs/book_re.aspx"
)

var account = flag.String("u", "", "账号")
var password = flag.String("p", "", "密码")

var httpClient http.Client
var meatPosts = []meatPost{}
var arrayIndex = 0

func init() {
	checkParams()
	initHTTPClient()
}

func checkParams() {
	flag.Parse()
	if *account == "" || *password == "" {
		log.Fatal("缺少参数,usage: -h")
	}
}

func initHTTPClient() {
	jar, err := cookiejar.New(&cookiejar.Options{})
	errHandle(err, "cookiJar 出错")
	httpClient = http.Client{
		Jar: jar,
	}
}

func login() {
	log.Println("get 请求:", LoginURI)
	_, err := httpClient.Get(LoginURI)
	errHandle(err, "请求登录页面出错")

	parameters := url.Values{
		"logname": {*account},
		"logpass": {*password},
		"savesid": {"0"},
		"action":  {"login"},
		"classid": {"0"},
		"siteid":  {"1000"},
		"sid":     {"-2-0-0-0-0"},
		"g":       {"登 录"},
	}
	log.Println("post 请求:", LoginURI)
	resp, err := httpClient.PostForm(LoginURI, parameters)
	errHandle(err, "提交登录出错")
	body, _ := ioutil.ReadAll(resp.Body)
	resp.Body.Close()

	match, _ := regexp.Match("登录成功", body)

	if !match {
		log.Fatal(string(body))
	}

	log.Println("登录成功!")
}

func findMeatPostByListURL(listURL string) {
	page := "1"
	lpage := 1
	for {
		fmt.Printf("第%s页\n", page)
		resp, err := httpClient.Get(listURL)
		errHandle(err, fmt.Sprintf("请求 %s 出错", listURL))
		body, _ := ioutil.ReadAll(resp.Body)
		resp.Body.Close()

		r, _ := regexp.Compile("bt2.*<a href=\"(.*page=(\\d+))\">下一页")
		matchesByte := r.FindSubmatch(body)
		matchesURL := BaseURI + strings.Replace(string(matchesByte[1]), "&amp;", "&", -1)
		matchesPage := string(matchesByte[2])

		if matchesPage == page {
			log.Printf("已扫描当前分类下的所有文章, 当前页 %s, url: %s", matchesPage, matchesURL)
			break
		}

		r, _ = regexp.Compile("礼\"/><a href=\"(.*?)\"[^>]*>")
		matches := r.FindAllSubmatch(body, -1)

		u, err := url.Parse(matchesURL)
		if err != nil {
			errLog(err, fmt.Sprintf("解析错误: %s", matchesURL))
			continue
		}

		querys, _ := url.ParseQuery(u.RawQuery)

		classID, _ := strconv.Atoi(querys["classid"][0])
		if matchesPage != "2" {
			lpage, _ = strconv.Atoi(querys["page"][0])
		}
		for _, match := range matches {
			filterMeatPost(string(match[1]), classID, lpage)
		}

		page = matchesPage
		listURL = matchesURL
	}
}

func filterMeatPost(url string, classID, lpage int) {
	url = BaseURI + url
	resp, err := httpClient.Get(url)
	if err != nil {
		log.Println(fmt.Sprintf("请求肉贴 %s 出错", url))
		return
	}
	body, _ := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	r, _ := regexp.Compile("礼金：(\\d+)&nbsp;已派：(\\d+)\\(余(\\d+)\\).*每人每日一次派礼：(\\d+).*标题]([^\\(]*)")
	matchs := r.FindStringSubmatch(string(body))
	totalMoney, _ := strconv.Atoi(matchs[1])
	balance, _ := strconv.Atoi(matchs[3])

	if balance == 0 {
		return
	}

	assign, _ := strconv.Atoi(matchs[4])

	rID, _ := regexp.Compile("bbs-(\\d+)\\.html")
	matchsID := rID.FindStringSubmatch(url)
	ID, _ := strconv.Atoi(matchsID[1])
	meat := meatPost{
		urlStr:     url,
		title:      matchs[5],
		totalMoney: totalMoney,
		balance:    balance,
		assign:     assign,
		classid:    classID,
		lpage:      lpage,
		id:         ID,
	}
	reply(meat)
	meatPosts = append(meatPosts, meat)
}

func reply(meat meatPost) {
	log.Printf("当前帖子: %s(%s)\n余额: %d,派礼: %d\n", meat.title, meat.urlStr, meat.balance, meat.assign)

	u, _ := url.Parse(meat.urlStr)
	cookies := httpClient.Jar.Cookies(u)
	sid := ""
	for _, v := range cookies {
		if v.Name == "sidyaohuo" {
			sid = v.Value
		}
	}
	response, err := httpClient.PostForm(ReplyURI, url.Values{
		"action":  {"add"},
		"id":      {strconv.Itoa(meat.id)},
		"siteid":  {"1000"},
		"lpage":   {strconv.Itoa(meat.lpage)},
		"classid": {strconv.Itoa(meat.classid)},
		"sid":     {sid},
		"sendmsg": {"0"},
		"g":       {"快速回复"},
		"content": {randString()},
	})

	if err != nil {
		log.Println("回复失败, 可能帖子已关闭")
		return
	}
	body, _ := ioutil.ReadAll(response.Body)
	response.Body.Close()

	r, _ := regexp.Compile("<div class=\"tip\">.*返回主题")
	fmt.Println(string(r.Find(body)))
}

func randString() string {
	array := [...]string{
		"吃",
		"吃肉",
		"吃吃吃",
		"谢谢",
		"吃了",
		"感谢",
	}

	if arrayIndex+1 >= len(array) {
		arrayIndex = 0
	} else {
		arrayIndex++
	}

	return array[arrayIndex]
}

func errHandle(err error, message string) {
	if err != nil {
		log.Fatal(message, err)
	}
}

func errLog(err error, message string) {
	if err != nil {
		log.Println(message, err)
	}
}

var urlTea = BaseURI + "/bbs/list.aspx?classid=177"

type category struct {
	name,
	urlStr string
}

var categorys = [...]category{
	{"妖火茶馆", "https://yaohuo.me/bbs/list.aspx?classid=177"},
	{"悬赏问答", "https://yaohuo.me/bbs/list.aspx?classid=213"},
}

func chioceCategory() (urlStr string) {
	fmt.Println("请选择要抓取的分类(默认为1):")
	for k, v := range categorys {
		fmt.Printf("%d: %s\n", k, v.name)
	}
	input := ""
	fmt.Scanln(&input)
	fmt.Println(input)
	if input == "" || input != "1" && input != "0" {
		log.Fatal("选择错误")
	}
	index, _ := strconv.Atoi(input)
	urlStr = categorys[index].urlStr
	return
}

// Run  启动程序
func Run() {
	login()
	urlStr := chioceCategory()
	findMeatPostByListURL(urlStr)
}

func main() {
	Run()
}
