package goinside

import (
	"errors"
	"fmt"
	"io/ioutil"
	"regexp"
	"strings"
)

var (
	filenameRe = regexp.MustCompile(`image/(.*)`)
)

func fetchSomething(formMap map[string]string, api dcinsideAPI, data interface{}) (err error) {
	resp, err := api.get(formMap)
	if err != nil {
		return
	}
	valid := make(jsonValidation, 1)
	if err = responseUnmarshal(resp, data, &valid); err != nil {
		return
	}
	if err = checkJSONResult(&valid); err != nil {
		return
	}
	return
}

type jsonGallery []struct {
	Category    string `json:"category"`
	ID          string `json:"name"`
	Name        string `json:"ko_name"`
	Number      string `json:"no"`
	Depth       string `json:"depth"`
	CanWrite    bool   `json:"no_write"`
	IsAdultOnly bool   `json:"is_adult"`
}

// FetchAllMajorGallery 함수는 모든 일반 갤러리의 정보를 가져옵니다.
func FetchAllMajorGallery() (mg []*MajorGallery, err error) {
	resp, err := majorGalleryListAPI.getWithoutHash()
	if err != nil {
		return
	}
	jsonResp := jsonGallery{}
	if err = responseUnmarshal(resp, &jsonResp); err != nil {
		return
	}
	mg = make([]*MajorGallery, len(jsonResp))
	for i, v := range jsonResp {
		mg[i] = &MajorGallery{
			ID:       v.ID,
			Name:     v.Name,
			Number:   v.Number,
			CanWrite: !v.CanWrite,
		}
	}
	return
}

type jsonMonirGallery []struct {
	Category    string `json:"category"`
	ID          string `json:"name"`
	Name        string `json:"ko_name"`
	Number      string `json:"no"`
	Depth       string `json:"depth"`
	CanWrite    bool   `json:"no_write"`
	IsAdultOnly bool   `json:"is_adult"`
	Manager     string `json:"manager"`
	SubManagers string `json:"submanager"`
}

// FetchAllMinorGallery 함수는 모든 마이너 갤러리의 정보를 가져옵니다.
func FetchAllMinorGallery() (mg []*MinorGallery, err error) {
	resp, err := minorGalleryListAPI.getWithoutHash()
	if err != nil {
		return
	}
	jsonResp := jsonMonirGallery{}
	if err = responseUnmarshal(resp, &jsonResp); err != nil {
		return
	}
	mg = make([]*MinorGallery, len(jsonResp))
	for i, v := range jsonResp {
		mg[i] = &MinorGallery{
			ID:          v.ID,
			Name:        v.Name,
			Number:      v.Number,
			CanWrite:    !v.CanWrite,
			Manager:     v.Manager,
			SubManagers: strings.Split(v.SubManagers, ","),
		}
	}
	return
}

type jsonList []struct {
	GallInfo []struct {
		CategoryName string `json:"category_name"`
		FileCount    string `json:"file_cnt"`
		FileSize     string `json:"file_size"`
	} `json:"gall_info"`
	GallList []struct {
		Subject      string `json:"subject"`
		Name         string `json:"name"`
		Level        string `json:"level"`
		ImageIcon    string `json:"img_icon"`
		WinnertaIcon string `json:"winnerta_icon"`
		ThumbsUp     string `json:"recommend"`
		ThumbsUpIcon string `json:"recommend_icon"`
		IsBest       string `json:"best_chk"`
		Hit          string `json:"hit"`
		UserID       string `json:"user_id"`
		MemberIcon   string `json:"member_icon"`
		IP           string `json:"ip"`
		TotalComment string `json:"total_comment"`
		TotalVoice   string `json:"total_voice"`
		Number       string `json:"no"`
		Date         string `json:"date_time"`
	} `json:"gall_list"`
}

// FetchList 함수는 해당 갤러리의 해당 페이지에 있는 글의 목록을 가져옵니다.
func FetchList(gallID string, page int) (l *List, err error) {
	return fetchList(gallID, page, false)
}

// FetchBestList 함수는 해당 갤러리의 해당 페이지에 있는 개념글의 목록을 가져옵니다.
func FetchBestList(gallID string, page int) (l *List, err error) {
	return fetchList(gallID, page, true)
}

func fetchList(gallID string, page int, fetchBestPage bool) (l *List, err error) {
	URL := gallURL(gallID)
	gall := &Gall{ID: gallID, URL: URL}
	formMap := map[string]string{
		"app_id": dummyGuest.getAppID(),
		"id":     gallID,
		"page":   fmt.Sprint(page),
	}
	if fetchBestPage {
		formMap["recommend"] = "1"
	}
	respJSON := make(jsonList, 1)
	if err = fetchSomething(formMap, readListAPI, &respJSON); err != nil {
		return
	}
	r := respJSON[0]
	l = &List{
		Info: &ListInfo{
			CategoryName: r.GallInfo[0].CategoryName,
			FileCount:    r.GallInfo[0].FileCount,
			FileSize:     r.GallInfo[0].FileSize,
			Gall:         gall,
		},
		Items: []*ListItem{},
	}
	for _, a := range r.GallList {
		item := &ListItem{
			Gall:               gall,
			URL:                articleURL(gallID, a.Number),
			Subject:            a.Subject,
			Name:               a.Name,
			Level:              Level(a.Level),
			HasImage:           a.ImageIcon == "Y",
			ArticleType:        articleType(a.ImageIcon, a.IsBest),
			ThumbsUp:           mustAtoi(a.ThumbsUp),
			IsBest:             a.IsBest == "Y",
			Hit:                mustAtoi(a.Hit),
			GallogID:           a.UserID,
			GallogURL:          gallogURL(a.UserID),
			IP:                 a.IP,
			CommentLength:      mustAtoi(a.TotalComment),
			VoiceCommentLength: mustAtoi(a.TotalVoice),
			Number:             a.Number,
			Date:               dateFormatter(a.Date),
		}
		l.Items = append(l.Items, item)
	}
	return
}

// Fetch 메소드는 해당 글의 세부 정보(본문, 이미지 주소, 댓글)를 가져옵니다.
func (i *ListItem) Fetch() (*Article, error) {
	return FetchArticle(i.URL)
}

// FetchImageURLs 메소드는 해당 글의 이미지 주소의 슬라이스만을 가져옵니다.
func (i *ListItem) FetchImageURLs() (imageURLs []ImageURLType, err error) {
	formMap := map[string]string{
		"app_id": dummyGuest.getAppID(),
		"id":     i.Gall.ID,
		"no":     fmt.Sprint(i.Number),
	}
	images := make(jsonArticleImages, 1)
	err = fetchSomething(formMap, readArticleImageAPI, &images)
	if err != nil {
		return
	}
	imageURLs = func() (ret []ImageURLType) {
		for _, v := range images {
			ret = append(ret, ImageURLType(v.Image))
		}
		return
	}()
	return
}

// Fetch 메소드는 해당 이미지 주소를 참조하여 이미지의 []byte와 filename을 반환합니다.
func (i ImageURLType) Fetch() (data []byte, filename string, err error) {
	resp, err := doImage(i)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	contentType := resp.Header.Get("Content-Type")
	matched := filenameRe.FindStringSubmatch(contentType)
	if len(matched) != 2 {
		err = errors.New("cannot found filename from Content-Type")
		return
	}

	filename = strings.ToLower(matched[1])
	data, err = ioutil.ReadAll(resp.Body)
	return
}

type jsonArticle []struct {
	ViewInfo struct {
		GallTitle          string `json:"galltitle"`
		Category           string `json:"category"`
		Subject            string `json:"subject"`
		Number             string `json:"no"`
		Name               string `json:"name"`
		Level              string `json:"level"`
		MemberIcon         string `json:"member_icon"`
		TotalComment       string `json:"total_comment"`
		IP                 string `json:"ip"`
		HasImage           string `json:"img_chk"`
		IsBest             string `json:"recommend_chk"`
		IsWinnerta         string `json:"winnerta_chk"`
		HasVoice           string `json:"voice_chk"`
		Hit                string `json:"hit"`
		WriteType          string `json:"write_type"`
		UserID             string `json:"user_id"`
		PrevArticleNumber  string `json:"prev_link"`
		PrevArticleSubject string `json:"prev_subject"`
		HeadTitle          string `json:"headtitle"`
		NextArticleNumber  string `json:"next_link"`
		NextArticleSubject string `json:"next_subject"`
		BestCheck          string `json:"best_chk"` // ?
		IsNotice           string `json:"isNotice"`
		Date               string `json:"date_time"`
	} `json:"view_info"`
	ViewMain struct {
		Memo           string `json:"memo"`
		ThumbsUp       string `json:"recommend"`
		ThumbsUpMember string `json:"recommend_member"`
		ThumbsDown     string `json:"nonrecommend"`
	} `json:"view_main"`
}

type jsonArticleImages []struct {
	Image string `json:"img"`
	// ImageClone string `json:"img_clone"`
}

// FetchArticle 함수는 해당 글의 정보를 가져옵니다.
func FetchArticle(URL string) (a *Article, err error) {
	gallID := gallID(URL)
	gall := &Gall{ID: gallID, URL: gallURL(gallID)}
	formMap := map[string]string{
		"app_id": dummyGuest.getAppID(),
		"id":     gallID,
		"no":     articleNumber(URL),
	}

	view := make(jsonArticle, 1)
	images := make(jsonArticleImages, 1)

	ch := func() <-chan error {
		ch := make(chan error)
		go func() {
			ch <- fetchSomething(formMap, readArticleAPI, &view)
		}()
		go func() {
			fetchSomething(formMap, readArticleImageAPI, &images)
			ch <- nil
		}()
		return ch
	}()

	for i := 0; i < 2; i++ {
		if err := <-ch; err != nil {
			return nil, err
		}
	}

	v := view[0]

	article := &Article{
		Gall:          gall,
		URL:           articleURL(gallID, v.ViewInfo.Number),
		Subject:       v.ViewInfo.Subject,
		Content:       v.ViewMain.Memo,
		ThumbsUp:      mustAtoi(v.ViewMain.ThumbsUp) + mustAtoi(v.ViewMain.ThumbsUpMember),
		ThumbsDown:    mustAtoi(v.ViewMain.ThumbsDown),
		Name:          v.ViewInfo.Name,
		Number:        v.ViewInfo.Number,
		Level:         MemberType(mustAtoi(v.ViewInfo.MemberIcon)).Level(),
		IP:            v.ViewInfo.IP,
		CommentLength: mustAtoi(v.ViewInfo.TotalComment),
		HasImage:      v.ViewInfo.HasImage == "Y",
		Hit:           mustAtoi(v.ViewInfo.Hit),
		ArticleType:   articleType(v.ViewInfo.HasImage, v.ViewInfo.IsBest),
		GallogID:      v.ViewInfo.UserID,
		GallogURL:     gallogURL(v.ViewInfo.UserID),
		IsBest:        v.ViewInfo.IsBest == "Y",
		ImageURLs: func() (ret []ImageURLType) {
			for _, v := range images {
				ret = append(ret, ImageURLType(v.Image))
			}
			return
		}(),
		Comments: []*Comment{},
		Date:     dateFormatter(v.ViewInfo.Date),
	}
	if article.CommentLength > 0 {
		article.Comments, err = fetchComment(URL, article)
		if err != nil {
			return
		}
	}
	if article.Subject == "" {
		return nil, errors.New("본문을 가져올 수 없음")
	}
	return article, nil
}

type jsonComment []struct {
	CommentCount string `json:"total_comment"`
	TotalPage    string `json:"total_page"`
	NowPage      string `json:"re_page"`
	Comments     []struct {
		MemberIcon string `json:"member_icon"`
		IP         string `json:"ipData"`
		Name       string `json:"name"`
		UserID     string `json:"user_id"`
		Content    string `json:"comment_memo"`
		Number     string `json:"comment_no"`
		Date       string `json:"date_time"`
	} `json:"comment_list"`
}

func fetchComment(URL string, parents *Article) (cs []*Comment, err error) {
	gallID := gallID(URL)
	gallURL := gallURL(gallID)
	gall := &Gall{ID: gallID, URL: gallURL}
	cs = []*Comment{}
	for commentPage := 1; ; commentPage++ {
		formMap := map[string]string{
			"app_id":  dummyGuest.getAppID(),
			"id":      gallID,
			"no":      parents.Number,
			"re_page": fmt.Sprint(commentPage),
		}
		respJSON := make(jsonComment, 1)
		if err = fetchSomething(formMap, readCommentAPI, &respJSON); err != nil {
			return
		}
		r := respJSON[0]
		for _, c := range r.Comments {
			comment := &Comment{
				Gall:      gall,
				Parents:   parents,
				Name:      c.Name,
				GallogID:  c.UserID,
				GallogURL: gallogURL(c.UserID),
				IP:        c.IP,
				Number:    c.Number,
				Date:      dateFormatter(c.Date),
			}
			comment.Content, comment.HTML = c.Content, c.Content
			cs = append(cs, comment)
		}
		if mustAtoi(r.NowPage) >= mustAtoi(r.TotalPage) {
			break
		}
	}
	return
}
