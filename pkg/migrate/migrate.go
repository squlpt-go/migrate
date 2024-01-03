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
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
)

var defaultFileExtensions = []string{".sql"}
var defaultLockFile = ".migrate.lock.json"

type Config struct {
	Dirs           []string `json:"dirs"`
	FileExtensions []string `json:"file_extensions"`
	LockFile       string   `json:"lock_file"`
}

type Lock struct {
	Migrations []Result `json:"migrations"`
}

type Result struct {
	Filepath  string    `json:"filepath"`
	Timestamp time.Time `json:"timestamp"`
	Sum       string    `json:"sum"`
}

type Results []Result

func LoadConfigFile(filename string) (Config, error) {
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
	for i, dir := range config.Dirs {
		if !strings.HasPrefix(dir, "/") {
			config.Dirs[i] = path.Join(configDir, dir)
		}
	}

	if config.LockFile == "" {
		config.LockFile = defaultLockFile
	}

	if !strings.HasPrefix(config.LockFile, "/") {
		config.LockFile = path.Join(configDir, config.LockFile)
	}

	return config, nil
}

func Migrate(db *sql.DB, config Config) ([]Result, error) {
	lock, err := loadLockFile(config.LockFile)
	if err != nil {
		return nil, err
	}

	results, err := doMigrate(db, config, lock)
	if results != nil {
		lock.Migrations = append(lock.Migrations, results...)
		writeErr := writeLockFile(config.LockFile, lock)
		if writeErr != nil {
			panic("cannot write lock file: " + writeErr.Error())
		}
	}

	return results, err
}

func loadLockFile(filepath string) (Lock, error) {
	var lock Lock

	if _, err := os.Stat(filepath); errors.Is(err, os.ErrNotExist) {
		return Lock{}, nil
	}

	configFile, err := os.Open(filepath)
	if err != nil {
		return Lock{}, fmt.Errorf("error reading lockfile: %w", err)
	}

	jsonParser := json.NewDecoder(configFile)
	if err = jsonParser.Decode(&lock); err != nil {
		return Lock{}, fmt.Errorf("parsing lock file: %w", err)
	}

	return lock, nil
}

func writeLockFile(filepath string, lock Lock) error {
	data, err := json.MarshalIndent(lock, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath, data, 0644)
}

func doMigrate(db *sql.DB, config Config, lock Lock) ([]Result, error) {
	results := make([]Result, 0)
	for _, dir := range config.Dirs {
		r, err := migrateDir(db, config, lock, dir)
		if r != nil {
			results = append(results, r...)
		}
		if err != nil {
			return results, err
		}
	}

	return results, nil
}

func migrateDir(db *sql.DB, config Config, lock Lock, dir string) ([]Result, error) {
	files, err := getDirFiles(dir)
	if err != nil {
		return nil, err
	}

	fileExtensions := config.FileExtensions
	if fileExtensions == nil {
		fileExtensions = defaultFileExtensions
	}

	results := make([]Result, 0)
	for _, dirEntry := range files {
		if dirEntry.IsDir() {
			continue
		}

		ext := filepath.Ext(dirEntry.Name())

		if !slices.Contains(fileExtensions, ext) {
			continue
		}

		fp := path.Join(dir, dirEntry.Name())

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

func lockHasFile(lock Lock, filepath string) bool {
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
