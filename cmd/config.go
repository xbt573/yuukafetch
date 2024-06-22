package cmd

type Config struct {
	Verbose   bool       `yaml:"verbose"`
	Quiet     bool       `yaml:"quiet"`
	Chooser   []string   `yaml:"chooser"`
	ApiKey    string     `yaml:"api_key"`
	UserId    int        `yaml:"user_id"`
	Downloads []Download `yaml:"downloads"`
}

type Download struct {
	Name         string   `yaml:"name"`
	Autodownload bool     `yaml:"autodownload"`
	Tags         string   `yaml:"tags"`
	LastId       int      `yaml:"lastid"`
	OutputDir    string   `yaml:"outputdir"`
	PickDir      string   `yaml:"pickdir"`
	CheckDirs    []string `yaml:"checkdirs"`
}
