package migrate

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

var DefaultLockFile = ".migrate.lock.json"
var NoLockFile = "/NO/LOCK/FILE/"

type Config struct {
	Paths          []string `json:"paths"`
	LockFile       string   `json:"lock_file"`
	IgnoreLockFile bool
}

func (c *Config) AddPath(paths ...string) {
	c.Paths = append(c.Paths, paths...)
}

func (c *Config) Merge(config Config) {
	c.AddPath(config.Paths...)
}

func NewConfig(dir string, paths []string) Config {
	c := Config{
		LockFile: path.Join(dir, DefaultLockFile),
	}
	for _, p := range paths {
		c.AddPath(path.Join(dir, p))
	}
	return c
}

type lock struct {
	Migrations []Result `json:"migrations"`
}

type Result struct {
	Filepath  string    `json:"filepath"`
	Timestamp time.Time `json:"timestamp"`
	Sum       string    `json:"sum"`
}

type Results []Result

func Migrate(db *sql.DB, config Config) ([]Result, error) {
	var l lock

	if !config.IgnoreLockFile {
		var err error
		l, err = loadLockFile(config.LockFile)
		if err != nil {
			return nil, err
		}
	}

	results, err := doMigrate(db, config, l)
	if results != nil {
		l.Migrations = append(l.Migrations, results...)

		if !config.IgnoreLockFile {
			writeErr := writeLockFile(config.LockFile, l)
			if writeErr != nil {
				panic("cannot write lock file: " + writeErr.Error())
			}
		}
	}

	return results, err
}

func loadConfigFile(filename string) (Config, error) {
	var config Config

	configFile, err := os.Open(filename)
	if err != nil {
		return Config{}, fmt.Errorf("error reading config file: %w", err)
	}

	jsonParser := json.NewDecoder(configFile)
	if err = jsonParser.Decode(&config); err != nil {
		return Config{}, fmt.Errorf("parsing config file: %w", err)
	}

	configDir := path.Dir(filename)
	for i, p := range config.Paths {
		if !strings.HasPrefix(p, "/") {
			config.Paths[i] = path.Join(configDir, p)
		}
	}

	if config.LockFile == "" {
		config.LockFile = DefaultLockFile
	}

	if !strings.HasPrefix(config.LockFile, "/") {
		config.LockFile = path.Join(configDir, config.LockFile)
	}

	return config, nil
}

func loadLockFile(filepath string) (lock, error) {
	var l lock

	if _, err := os.Stat(filepath); errors.Is(err, os.ErrNotExist) {
		return lock{}, nil
	}

	configFile, err := os.Open(filepath)
	if err != nil {
		return lock{}, fmt.Errorf("error reading lockfile: %w", err)
	}

	jsonParser := json.NewDecoder(configFile)
	if err = jsonParser.Decode(&l); err != nil {
		return lock{}, fmt.Errorf("parsing lock file: %w", err)
	}

	return l, nil
}

func writeLockFile(filepath string, lock lock) error {
	data, err := json.MarshalIndent(lock, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath, data, 0644)
}

func doMigrate(db *sql.DB, config Config, lock lock) ([]Result, error) {
	results := make([]Result, 0)
	for _, glob := range config.Paths {
		r, err := migrateGlob(db, lock, glob)
		if r != nil {
			results = append(results, r...)
		}
		if err != nil {
			return results, err
		}
	}

	return results, nil
}

func migrateGlob(db *sql.DB, lock lock, glob string) ([]Result, error) {
	files, err := filepath.Glob(glob)
	if err != nil {
		return nil, err
	}

	results := make([]Result, 0)
	for _, fp := range files {
		if !lockHasFile(lock, fp) {
			sum, err := calculateSum(fp)
			if err != nil {
				return results, fmt.Errorf("error calculating checksum for migration file %#v: %w", fp, err)
			}
			qs, err := os.ReadFile(fp)
			if err != nil {
				return results, fmt.Errorf("error reading migration file %#v: %w", fp, err)
			}
			_, err = db.Exec(string(qs))
			if err != nil {
				return results, fmt.Errorf("error executing migration file %#v: %w", fp, err)
			}
			results = append(results, Result{
				Filepath:  fp,
				Timestamp: time.Now(),
				Sum:       sum,
			})
		}
	}

	return results, nil
}

func getDirFiles(dir string) ([]os.DirEntry, error) {
	// Read the directory
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	// Define a regular expression to check if the first segment is a number
	numRegex := regexp.MustCompile(`^\d+`)

	// Sort the list based on custom criteria
	sort.Slice(files, func(i, j int) bool {
		name1 := files[i].Name()
		name2 := files[j].Name()

		// Extract the first segment of each filename
		num1, isNum1 := extractNumber(name1, numRegex)
		num2, isNum2 := extractNumber(name2, numRegex)

		// Compare numerically if both are numbers, otherwise compare alphabetically
		if isNum1 && isNum2 {
			return num1 < num2
		} else {
			return name1 < name2
		}
	})

	return files, nil
}

func extractNumber(s string, regex *regexp.Regexp) (int, bool) {
	match := regex.FindString(s)
	if match == "" {
		return 0, false
	}

	num, err := strconv.Atoi(match)
	if err != nil {
		return 0, false
	}

	return num, true
}

func lockHasFile(lock lock, filepath string) bool {
	for _, result := range lock.Migrations {
		if filepath == result.Filepath {
			return true
		}
	}

	return false
}

func calculateSum(filepath string) (string, error) {
	// Open the file
	file, err := os.Open(filepath)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()

	// Create a new SHA-256 hash
	hash := sha256.New()

	// Copy the file contents into the hash calculator
	_, err = io.Copy(hash, file)
	if err != nil {
		return "", err
	}

	// Get the hash sum as a byte slice
	hashInBytes := hash.Sum(nil)

	// Convert the byte slice to a hexadecimal string
	hashString := hex.EncodeToString(hashInBytes)

	return hashString, nil
}
