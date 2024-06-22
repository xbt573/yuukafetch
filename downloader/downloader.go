package downloader

import (
	"context"
	"io"
	"net/http"
	"os"
	"path"
	"runtime"
	"sync"

	"github.com/xbt573/yuukafetch/gelbooru"
	"github.com/xbt573/yuukafetch/logger"
	"github.com/xbt573/yuukafetch/progressbar"
	"golang.org/x/sync/semaphore"
)

type Downloader struct {
	showProgressbar     bool
	tags                string
	outputDir           string
	checkDirs           []string
	gelbooruOptions     gelbooru.GelbooruOptions
	progressbarInstance *progressbar.Progressbar
	sem                 *semaphore.Weighted
	wg                  sync.WaitGroup
}

type DownloaderOptions struct {
	ShowProgressbar bool
	OutputDir       string
	CheckDirs       []string
	Tags            string
	GelbooruOptions gelbooru.GelbooruOptions
}

func NewDownloader(options DownloaderOptions) *Downloader {
	return &Downloader{
		showProgressbar: options.ShowProgressbar,
		tags:            options.Tags,
		outputDir:       options.OutputDir,
		checkDirs:       options.CheckDirs,
		gelbooruOptions: options.GelbooruOptions,
		sem:             semaphore.NewWeighted(int64(runtime.NumCPU())),
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

func (d *Downloader) Download() error {
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
	count := res.Attributes.Count
	errch := make(chan error, 1)

	ctx, cancel := context.WithCancel(context.Background())

	if d.progressbarInstance != nil {
		go d.progressbarInstance.Start(ctx)
	}

	for pid := 0; ; pid++ {
		select {
		case err := <-errch:
			cancel()

			logger.Errorf("downloader: error: %s", err.Error())
			logger.Error("downloader: ensure your directory exists if you have \"no such file or directory\" error")
			os.Exit(1)
		default:
		}

		logger.Logf("downloader: fetching page %d", pid+1) // normalizing page id for normal people
		res, err := gelbooruInstance.Fetch(d.tags, pid, 0)
		if err != nil {
			cancel() // fucking warnings
			return err
		}

		if len(res.Post) == 0 {
			logger.Log("downloader: finishing")
			break
		}

		for _, post := range res.Post {
			select {
			case err := <-errch:
				cancel()

				logger.Errorf("downloader: error: %s", err.Error())
				logger.Error("downloader: ensure your directory exists if you have \"no such file or directory\" error")
				os.Exit(1)
			default:
			}

			err := d.sem.Acquire(context.Background(), 1)
			if err != nil {
				cancel() // fucking warnings
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

		// donech := make(chan any)
		// go func() {
		// 	d.wg.Done()
		// 	donech <- nil
		// }()

		// select {
		// case err := <-errch:
		// 	cancel()

		// 	logger.Errorf("downloader: error: %s", err.Error())
		// 	logger.Error("downloader: ensure your directory exists if you have \"no such file or directory\" error")
		// 	os.Exit(1)
		// case <-donech:
		// }
	}

	d.wg.Wait()
	cancel() // idk why but it fixes the error

	if d.progressbarInstance != nil {
		d.progressbarInstance.Clear()
	}

	return nil
}
