package goinside

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"regexp"
)

var (
	flDataRe  = regexp.MustCompile(`\('FL_DATA'\).value ?= ?'(.*)'`)
	oflDataRe = regexp.MustCompile(`\('OFL_DATA'\).value ?= ?'(.*)'`)
	urlRe     = regexp.MustCompile(`url="?(.*?)"?>`)
	idRe      = regexp.MustCompile(`id=([^&]*)`)
	numberRe  = regexp.MustCompile(`no=(\d+)`)
)

// NewArticle 함수는 새로운 NewArticleWriter 객체를 반환합니다.
func (s *Session) NewArticle(gallID, subject, content string, images ...string) *ArticleWriter {
	return &ArticleWriter{
		Session: s,
		GallID:  gallID,
		Subject: subject,
		Content: content,
		Images:  images,
	}
}

// Write 함수는 ArticleWriter의 정보를 가지고 글을 작성합니다.
func (a *ArticleWriter) Write() (*Article, error) {
	// get cookies and block key
	cookies, authKey, err := a.getCookiesAndAuthKey(map[string]string{
		"id":        "programming",
		"w_subject": a.Subject,
		"w_memo":    a.Content,
		"w_filter":  "1",
		"mode":      "write_verify",
	}, OptionWriteURL)
	if err != nil {
		return nil, err
	}

	// upload images and get FL_DATA, OFL_DATA string
	var flData, oflData string
	if len(a.Images) > 0 {
		flData, oflData, err = a.uploadImages(a.GallID, a.Images)
		if err != nil {
			return nil, err
		}
	}

	// wrtie article
	ret := &Article{}
	form, contentType := multipartForm(nil, map[string]string{
		"name":       a.id,
		"password":   a.pw,
		"subject":    a.Subject,
		"memo":       a.Content,
		"mode":       "write",
		"id":         a.GallID,
		"mobile_key": "mobile_nomember",
		"FL_DATA":    flData,
		"OFL_DATA":   oflData,
		"Block_key":  authKey,
		"filter":     "1",
	})
	resp, err := a.post(GWriteURL, cookies, form, contentType)
	if err != nil {
		return nil, err
	}
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	body := string(bodyBytes)
	URL := urlRe.FindStringSubmatch(body)
	gallID := idRe.FindStringSubmatch(body)
	number := numberRe.FindStringSubmatch(body)
	if len(URL) != 2 || len(gallID) != 2 || len(number) != 2 {
		return nil, errors.New("Write Article Fail")
	}
	ret.Gall.URL, ret.Gall.ID, ret.Number = URL[1], gallID[1], number[1]
	return ret, nil
}

// DeleteArticle 함수는 인자로 주어진 글을 삭제합니다.
func (s *Session) DeleteArticle(a *Article) error {
	// get cookies and con key
	m := map[string]string{}
	if s.nomember {
		m["token_verify"] = "nonuser_del"
	} else {
		return errors.New("Need to login")
	}
	cookies, authKey, err := s.getCookiesAndAuthKey(m, AccessTokenURL)
	if err != nil {
		return err
	}

	// delete article
	form := form(map[string]string{
		"id":       a.Gall.ID,
		"write_pw": s.pw,
		"no":       a.Number,
		"mode":     "board_del2",
		"con_key":  authKey,
	})
	_, err = s.post(OptionWriteURL, cookies, form, DefaultContentType)
	return err
}

// DeleteArticleAll 함수는 인자로 주어진 여러 개의 글을 동시에 삭제합니다.
func (s *Session) DeleteArticleAll(as []*Article) error {
	done := make(chan error)
	defer close(done)
	for _, a := range as {
		a := a
		go func() {
			done <- s.DeleteArticle(a)
		}()
	}
	for _ = range as {
		if err := <-done; err != nil {
			return err
		}
	}
	return nil
}

func (s *Session) uploadImages(gall string, images []string) (string, string, error) {
	form, contentType := multipartForm(images, map[string]string{
		"imgId":   gall,
		"mode":    "write",
		"img_num": "11", // ?
	})
	resp, err := s.post(UploadImageURL, nil, form, contentType)
	if err != nil {
		return "", "", err
	}
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", "", err
	}
	body := string(bodyBytes)
	fldata := flDataRe.FindStringSubmatch(body)
	ofldata := oflDataRe.FindStringSubmatch(body)
	if len(fldata) != 2 || len(ofldata) != 2 {
		return "", "", errors.New("Image Upload Fail")
	}
	return fldata[1], ofldata[1], nil
}

func (s *Session) getCookiesAndAuthKey(m map[string]string, URL string) ([]*http.Cookie, string, error) {
	var cookies []*http.Cookie
	var authKey string
	form := form(m)
	resp, err := s.post(URL, nil, form, DefaultContentType)
	if err != nil {
		return nil, "", err
	}
	cookies = resp.Cookies()
	authKey, err = parseAuthKey(resp)
	if err != nil {
		return nil, "", err
	}
	return cookies, authKey, nil
}

func parseAuthKey(resp *http.Response) (string, error) {
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var tempJSON struct {
		Msg  string
		Data string
	}
	json.Unmarshal(body, &tempJSON)
	if tempJSON.Data == "" {
		return "", errors.New("Block Key Parse Fail")
	}
	return tempJSON.Data, nil
}

func multipartForm(images []string, m map[string]string) (io.Reader, string) {
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	if images != nil {
		multipartImages(w, images)
	}
	multipartOthers(w, m)
	return &b, w.FormDataContentType()
}

func multipartImages(w *multipart.Writer, images []string) {
	for i, image := range images {
		f, err := os.Open(image)
		if err != nil {
			return
		}
		defer f.Close()
		fw, err := w.CreateFormFile(fmt.Sprintf("upload[%d]", i), image)
		if err != nil {
			return
		}
		if _, err = io.Copy(fw, f); err != nil {
			return
		}
	}
}

func multipartOthers(w *multipart.Writer, m map[string]string) {
	for k, v := range m {
		if fw, err := w.CreateFormField(k); err != nil {
			continue
		} else if _, err := fw.Write([]byte(v)); err != nil {
			continue
		}
	}
}

// NewComment 함수는 새로운 CommentWriter 객체를 반환합니다.
func (s *Session) NewComment(a *Article, content string) *CommentWriter {
	return &CommentWriter{
		Session: s,
		Article: a,
		Content: content,
	}
}

// WriteComment 함수는 CommentWriter의 정보를 가지고 댓글을 작성합니다.
func (c *CommentWriter) Write() (*Comment, error) {
	form := form(map[string]string{
		"id":           c.Gall.ID,
		"no":           c.Number,
		"ip":           c.ip,
		"comment_nick": c.id,
		"comment_pw":   c.pw,
		"comment_memo": c.Content,
		"mode":         "comment_nonmember",
	})
	resp, err := c.post(CommentURL, nil, form, DefaultContentType)
	if err != nil {
		return nil, err
	}
	commentNumber, err := parseCommentNumber(resp)
	if err != nil {
		return nil, err
	}
	URL := fmt.Sprintf("http://m.dcinside.com/view.php?id=%s&no=%s",
		c.Gall.ID, c.Number)
	return &Comment{
		Gall:    &GallInfo{URL: URL, ID: c.Gall.ID},
		Parents: &Article{Number: c.Number},
		Number:  commentNumber,
	}, nil
}

// DeleteComment 함수는 인자로 주어진 댓글을 삭제합니다.
func (s *Session) DeleteComment(c *Comment) error {
	// get cookies and con key
	m := map[string]string{}
	if s.nomember {
		m["token_verify"] = "nonuser_com_del"
	} else {
		return errors.New("Need to login")
	}
	cookies, authKey, err := s.getCookiesAndAuthKey(m, AccessTokenURL)
	if err != nil {
		return err
	}

	// delete comment
	form := form(map[string]string{
		"id":         c.Gall.ID,
		"no":         c.Parents.Number,
		"iNo":        c.Number,
		"comment_pw": s.pw,
		"user_no":    "nonmember",
		"mode":       "comment_del",
		"con_key":    authKey,
	})
	_, err = s.post(OptionWriteURL, cookies, form, DefaultContentType)
	return err
}

// DeleteCommentAll 함수는 인자로 주어진 여러 개의 댓글을 동시에 삭제합니다.
func (s *Session) DeleteCommentAll(cs []*Comment) error {
	done := make(chan error)
	defer close(done)
	for _, c := range cs {
		c := c
		go func() {
			done <- s.DeleteComment(c)
		}()
	}
	for _ = range cs {
		if err := <-done; err != nil {
			return err
		}
	}
	return nil
}

func parseCommentNumber(resp *http.Response) (string, error) {
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var tempJSON struct {
		Msg  string
		Data string
	}
	json.Unmarshal(body, &tempJSON)
	if tempJSON.Data == "" {
		return "", errors.New("Block Key Parse Fail")
	}
	return tempJSON.Data, nil
}
