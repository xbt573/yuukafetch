package downloader

import (
	"context"
	"io"
	"net/http"
	"os"
	"path"
	"sync"

	"github.com/xbt573/yuukafetch/gelbooru"
	"github.com/xbt573/yuukafetch/logger"
	"github.com/xbt573/yuukafetch/progressbar"
	"golang.org/x/sync/semaphore"
)

type Downloader struct {
	showProgressbar     bool
	tags                string
	lastid              int
	outputDir           string
	checkDirs           []string
	gelbooruOptions     gelbooru.GelbooruOptions
	progressbarInstance *progressbar.Progressbar
	sem                 *semaphore.Weighted
	wg                  sync.WaitGroup
}

type DownloaderOptions struct {
	ShowProgressbar bool
	Tags            string
	LastId          int
	OutputDir       string
	CheckDirs       []string
	GelbooruOptions gelbooru.GelbooruOptions
	DownloadThreads uint
}

func NewDownloader(options DownloaderOptions) *Downloader {
	return &Downloader{
		showProgressbar: options.ShowProgressbar,
		tags:            options.Tags,
		lastid:          options.LastId,
		outputDir:       options.OutputDir,
		checkDirs:       options.CheckDirs,
		gelbooruOptions: options.GelbooruOptions,
		sem:             semaphore.NewWeighted(int64(options.DownloadThreads)),
	}
}

func (d *Downloader) downloadFile(url, path string) error {
	res, err := http.Get(url)
	if err != nil {
		return err
	}

	defer res.Body.Close()

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, res.Body)
	if err != nil {
		return err
	}

	return nil
}

func (d *Downloader) exists(file, outputDir string, checkDirs []string) bool {
	if _, err := os.Stat(path.Join(outputDir, file)); err == nil {
		return true
	}

	for _, dir := range checkDirs {
		if _, err := os.Stat(path.Join(dir, file)); err == nil {
			return true
		}
	}

	return false
}

// shameful peace of code, because of error handling
func (d *Downloader) Download(ctx context.Context) (err error) {
	gelbooruInstance := gelbooru.NewGelbooru(d.gelbooruOptions)

	res, err := gelbooruInstance.Fetch(d.tags, 0, 1)
	if err != nil {
		return err
	}
	logger.Logf("downloader: total count is %d", res.Attributes.Count)

	if d.showProgressbar {
		d.progressbarInstance = progressbar.NewProgressbar(res.Attributes.Count)
	}

	current := 0
	count := res.Attributes.Count // то что он не юзается — пиздёж
	errch := make(chan error, 1)

	progressbarCtx, cancel := context.WithCancel(context.Background())

	if d.progressbarInstance != nil {
		go d.progressbarInstance.Start(progressbarCtx)
	}

pidloop:
	for pid := 0; ; pid++ {
		select {
		case err = <-errch:
			logger.Errorf("downloader: error: %s", err.Error())
			logger.Error("downloader: ensure your directory exists if you have \"no such file or directory\" error")
			break pidloop
		case <-ctx.Done():
			logger.Log("downloader: finishing")
			break pidloop
		default:
		}

		logger.Logf("downloader: fetching page %d", pid+1) // normalizing page id for normal people
		res, err := gelbooruInstance.Fetch(d.tags, pid, 0)
		if err != nil {
			cancel() // ну хуесосина блять
			return err
		}

		if len(res.Post) == 0 {
			logger.Log("downloader: finishing")
			break
		}

		for _, post := range res.Post {
			if post.Id < d.lastid {
				logger.Log("downloader: reached lastid, finishing")
				break pidloop
			}

			select {
			case err = <-errch:
				logger.Errorf("downloader: error: %s", err.Error())
				logger.Error("downloader: ensure your directory exists if you have \"no such file or directory\" error")
				break pidloop
			case <-ctx.Done():
				logger.Log("downloader: finishing")
				break pidloop
			default:
			}

			err := d.sem.Acquire(context.Background(), 1)
			if err != nil {
				cancel() // ну хуесосина блять
				return err
			}

			d.wg.Add(1)

			go func(post gelbooru.Post) {
				defer d.wg.Done()
				defer d.sem.Release(1)

				exists := d.exists(post.Image, d.outputDir, d.checkDirs)
				if exists {
					current++

					if d.progressbarInstance != nil {
						d.progressbarInstance.Add(post.Image)
					}

					logger.Logf("downloader: found %s, %d/%d", post.Image, current, count)
					return
				}

				err := d.downloadFile(post.FileUrl, path.Join(d.outputDir, post.Image))
				if err != nil {
					errch <- err
					return
				}

				current++

				if d.progressbarInstance != nil {
					d.progressbarInstance.Add(post.Image)
				}

				logger.Logf("downloader: downloaded %s, %d/%d", post.Image, current, count)
			}(post)
		}
	}

	d.wg.Wait()
	cancel()

	if d.progressbarInstance != nil {
		d.progressbarInstance.Clear()
	}

	return
}
