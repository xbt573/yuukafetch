package cmd

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"runtime"
	"runtime/debug"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/xbt573/yuukafetch/downloader"
	"github.com/xbt573/yuukafetch/gelbooru"
	"github.com/xbt573/yuukafetch/logger"
)

var (
	config       string
	configStruct Config
	version      = "undefined"
)

func getVersion() string {
	if version != "undefined" {
		return version
	}

	// set vcs version
	var revision string

	info, ok := debug.ReadBuildInfo()
	if ok {
		for _, setting := range info.Settings {
			switch setting.Key {
			case "vcs.revision":
				revision = setting.Value
			}
		}
	}

	if revision != "" {
		return revision
	}

	return version
}

var rootCmd = &cobra.Command{
	Use:     "yuukafetch command [options]",
	Version: getVersion(),
}

var fetchCmd = &cobra.Command{
	Use:   "fetch [options]",
	Short: "Fetch arts from Gelbooru",
	Run: func(cmd *cobra.Command, args []string) {
		logger.Initialize(configStruct.Verbose && !configStruct.Quiet, configStruct.Quiet)
		logger.Log("yuukafetch: starting")

		downloadProfile := viper.GetString("download")

		if len(configStruct.Downloads) == 0 {
			logger.Error("yuukafetch: no downloads found, check your config file")
			os.Exit(1)
		}

		for _, download := range configStruct.Downloads {
			isDefinedProfile := false
			skipping := false

			if downloadProfile != "" && download.Name == downloadProfile {
				isDefinedProfile = true
			}

			if downloadProfile != "" && !isDefinedProfile {
				skipping = true
			}

			if !skipping && !isDefinedProfile && !download.Autodownload {
				skipping = true
			}

			if skipping {
				logger.Printf("Skipping \"%v\"\n", download.Name)
				logger.Logf("yuukafetch: Skipping %s", download.Name)
				continue
			}

			logger.Printf("Downloading \"%v\"\n", download.Name)
			logger.Logf("yuukafetch: downloading %s", download.Name)

			downloaderInstance := downloader.NewDownloader(downloader.DownloaderOptions{
				ShowProgressbar: !configStruct.Verbose && !configStruct.Quiet,
				OutputDir:       download.OutputDir,
				CheckDirs:       download.CheckDirs,
				Tags:            download.Tags,
				LastId:          download.LastId,
				DownloadThreads: configStruct.Threads,
				GelbooruOptions: gelbooru.GelbooruOptions{
					ApiKey: configStruct.ApiKey,
					UserId: configStruct.UserId,
				},
			})

			errch := make(chan error)
			sigch := make(chan os.Signal, 10)
			donech := make(chan any)
			ctx, cancel := context.WithCancel(context.Background())

			signal.Notify(sigch, os.Interrupt)

			go func() {
				err := downloaderInstance.Download(ctx)
				if err != nil {
					errch <- err
				}
				cancel()
				donech <- nil
			}()

			errored := false
			finishing := false

			select {
			case err := <-errch:
				if err != nil {
					logger.Errorf("yuukafetch: downloader failed, %s", err.Error())
					errored = true
				}
			case <-sigch:
				logger.Log("yuukafetch: caught interrupt, finishing")
				cancel()

				finishing = true
			case <-donech:
			}

			if finishing || errored {
				select {
				case <-donech:
				case <-sigch:
					break
				}
			}

			if errored {
				os.Exit(1)
			}

			if finishing {
				return
			}
		}
	},
}

var pickCmd = &cobra.Command{
	Use:   "pick download",
	Short: "Pick images into pick directory",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		logger.Initialize(configStruct.Verbose && !configStruct.Quiet, configStruct.Quiet)
		logger.Log("yuukafetch: starting")

		name := args[0]

		if len(configStruct.Downloads) == 0 {
			logger.Error("yuukafetch: no downloads found, check your config file")
			os.Exit(1)
		}

		found := false

		for _, download := range configStruct.Downloads {
			if download.Name != name {
				continue
			}

			found = true

			files, err := os.ReadDir(download.OutputDir)
			if err != nil {
				logger.Errorf("yuukafetch: error while getting files: %s", err.Error())
				logger.Log("yuukafetch: ensure that output dir exist")
				os.Exit(1)
			}

			rand.Shuffle(len(files), func(i, j int) {
				files[i], files[j] = files[j], files[i]
			})

			for _, file := range files {
				filePath := path.Join(download.OutputDir, file.Name())
				pickPath := path.Join(download.PickDir, file.Name())

				cmd := append(configStruct.Chooser, filePath)
				command := cmd[0]
				args := cmd[1:]

				err := exec.Command(command, args...).Run()
				if err != nil {
					logger.Errorf("yuukafetch: error while launching: %s", err.Error())
					logger.Errorf("yuukafetch: command line used: %s", strings.Join(cmd, " "))
					os.Exit(1)
				}

			chooseloop:
				for {
					fmt.Printf("Keep %s? (Y/N): ", file.Name())

					choice := ""
					fmt.Scanln(&choice)

					switch choice {
					case "Y":
						fallthrough
					case "y":
						err := os.Rename(filePath, pickPath)
						if err != nil {
							logger.Errorf("yuukafetch: error moving file files: %s", err.Error())
							logger.Log("yuukafetch: ensure that outputdir and pickdir exist")
							os.Exit(1)
						}
						break chooseloop

					case "N":
						fallthrough
					case "n":
						break chooseloop

					default:
						fmt.Printf("Unknown choice: \"%v\"\n", choice)
					}
				}
			}
		}

		if !found {
			logger.Errorf("yuukafetch: unknown download \"%v\"\n", name)
		}
	},
}

func init() {
	logger.Initialize(true, false)
	rootCmd.AddCommand(fetchCmd)
	rootCmd.AddCommand(pickCmd)

	rootCmd.PersistentFlags().StringVarP(&config, "config", "c", "", "Path to config")
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Be very verbose (disables progressbar)")
	rootCmd.PersistentFlags().BoolP("quiet", "q", false, "Be very quiet (disables progressbar)")
	rootCmd.PersistentFlags().String("api_key", "", "Gelbooru API Key")
	rootCmd.PersistentFlags().Int("user_id", 0, "Gelbooru User ID")

	fetchCmd.PersistentFlags().StringP("download", "d", "", "Download specific download profile")
	fetchCmd.PersistentFlags().UintP("threads", "t", 5, "Number of threads to download arts")

	var chooser []string

	switch runtime.GOOS {
	case "freebsd":
		fallthrough
	case "openbsd":
		fallthrough
	case "linux":
		chooser = []string{"feh", "-."}

	default:
		panic(fmt.Sprintf("currently unsupported os: %s. create issue to (maybe) fix it", runtime.GOOS))
	}

	pickCmd.PersistentFlags().StringSlice("chooser", chooser, "Chooser cmd (image should last argument)")

	viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))
	viper.BindPFlag("quiet", rootCmd.PersistentFlags().Lookup("quiet"))
	viper.BindPFlag("api_key", rootCmd.PersistentFlags().Lookup("api_key"))
	viper.BindPFlag("user_id", rootCmd.PersistentFlags().Lookup("user_id"))

	viper.BindPFlag("download", fetchCmd.PersistentFlags().Lookup("download"))
	viper.BindPFlag("threads", fetchCmd.PersistentFlags().Lookup("threads"))

	viper.BindPFlag("chooser", pickCmd.PersistentFlags().Lookup("chooser"))

	cobra.OnInitialize(func() {
		viper.AddConfigPath(".")
		viper.AddConfigPath("$XDG_CONFIG_HOME/yuukafetch")
		viper.AddConfigPath("$HOME/.config/yuukafetch")
		viper.AddConfigPath("/etc/yuukafetch")
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")

		if config != "" {
			viper.SetConfigFile(config)
		}

		if err := viper.ReadInConfig(); err != nil {
			if _, ok := err.(viper.ConfigFileNotFoundError); ok {
				logger.Errorf("yuukafetch: can't found config, %s", err.Error())
				return
			} else {
				logger.Errorf("yuukafetch: config error, %s", err.Error())
				os.Exit(1)
			}
		}

		// shadows var config string
		config := &Config{}
		err := viper.Unmarshal(config)
		if err != nil {
			logger.Errorf("yuukafetch: can't unmarshal config, %s", err.Error())
			os.Exit(1)
		}

		configStruct = *config
	})
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
