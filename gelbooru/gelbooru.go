package gelbooru

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
)

var (
	ErrNotOK = errors.New("Gelbooru returned non-200 code")
)

type Gelbooru struct {
	apiKey string
	userId int
}

type GelbooruOptions struct {
	ApiKey string
	UserId int
}

func NewGelbooru(options ...GelbooruOptions) *Gelbooru {
	if len(options) == 0 {
		return &Gelbooru{}
	}

	return &Gelbooru{
		apiKey: options[0].ApiKey,
		userId: options[0].UserId,
	}
}

func (g *Gelbooru) Fetch(tags string, pageId, limit int) (*Response, error) {
	req, err := http.NewRequest(http.MethodGet, "https://gelbooru.com/index.php", nil)
	if err != nil {
		return nil, err
	}

	q := req.URL.Query()
	q.Add("page", "dapi")
	q.Add("s", "post")
	q.Add("q", "index")
	q.Add("json", "1")
	q.Add("tags", tags)
	q.Add("pid", strconv.Itoa(pageId))

	if limit != 0 {
		q.Add("limit", strconv.Itoa(limit))
	}

	if g.apiKey != "" {
		q.Add("api_key", g.apiKey)
	}

	if g.userId != 0 {
		q.Add("user_id", strconv.Itoa(g.userId))
	}

	req.URL.RawQuery = q.Encode()

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, ErrNotOK
	}

	var body Response
	err = json.NewDecoder(res.Body).Decode(&body)
	if err != nil {
		return nil, err
	}

	return &body, nil
}
