package gelbooru

// Sorry, this only realises types needed for fetcher to work
type Response struct {
	Attributes Attributes `json:"@attributes"`
	Post       []Post     `json:"post,omitempty"`
}

type Attributes struct {
	Count int `json:"count"`
}

type Post struct {
	Id      int    `json:"id"`
	Image   string `json:"image"`
	FileUrl string `json:"file_url"`
}
