package migrate

import (
	"database/sql"
	"errors"
	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"
)

func testPath(path string) string {
	return Path("/../../test/" + path)
}

var (
	testConfigStart   = testPath("config-start.json")
	testConfigIter    = testPath("config-iter.json")
	testConfigFailure = testPath("config-failure.json")
	testLock          = testPath("test-lock.json")
	testLockFailure   = testPath("test-lock-failure.json")
)

func getTestDB() *sql.DB {
	envFilePath := Path("/../../.migrate.env")
	err := godotenv.Load(envFilePath)
	if err != nil {
		panic(err)
	}
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		panic("must set DB_DSN env var")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		panic(err)
	}
	return db
}

func TestLoadConfigFile(t *testing.T) {
	config, err := loadConfigFile(testConfigIter)
	if err != nil {
		t.Fatalf("Error loading config file: %v", err)
	}

	expectedDirs := []string{testPath("start/*.sql"), testPath("iter/*.sql")}

	if !reflect.DeepEqual(config.Paths, expectedDirs) {
		t.Errorf("Unexpected Dirs in config. Expected: %v, Got: %v", expectedDirs, config.Paths)
	}
}

func TestConfig_AddPath(t *testing.T) {
	c := Config{}
	c.AddPath("/path1")
	if !reflect.DeepEqual(c.Paths, []string{"/path1"}) {
		t.Errorf("Unexpected Dirs in config. Got: %v", c.Paths)
	}
	c.AddPath("/path2", "/path3")
	if !reflect.DeepEqual(c.Paths, []string{"/path1", "/path2", "/path3"}) {
		t.Errorf("Unexpected Dirs in config. Got: %v", c.Paths)
	}
}

func TestConfig_Merge(t *testing.T) {
	c1 := Config{Paths: []string{"/path1", "/path2"}}
	c2 := Config{Paths: []string{"/path3", "/path4"}}
	c1.Merge(c2)
	if !reflect.DeepEqual(c1.Paths, []string{"/path1", "/path2", "/path3", "/path4"}) {
		t.Errorf("Unexpected Dirs in config. Got: %v", c1.Paths)
	}
}

func TestNewConfig(t *testing.T) {
	dir := Path("../../test")
	c := NewConfig(dir, []string{"start/*.sql", "iter/*.sql"})

	if !strings.HasSuffix(c.LockFile, DefaultLockFile) {
		t.Fatalf("should end in default lock file name")
	}

	if c.LockFile == DefaultLockFile {
		t.Fatalf("should not be the same as lock file name")
	}

	if len(c.Paths) != 2 {
		t.Fatalf("should nbe 2 paths")
	}

	if !strings.HasSuffix(c.Paths[0], "start/*.sql") || !strings.HasSuffix(c.Paths[1], "iter/*.sql") {
		t.Fatalf("invalid path values")
	}
}

func TestLoadConfigFileFails(t *testing.T) {
	_, err := loadConfigFile(testConfigFailure)
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestLoadLockFile(t *testing.T) {
	lock, err := loadLockFile(testLock)
	if err != nil {
		t.Fatalf("Error loading lock file: %v", err)
	}

	expectedFilepath := "/some/path.sql"
	expectedTimestamp, _ := time.Parse(time.RFC3339, "2006-01-02T15:04:05Z")
	expectedSum := "abc123"

	if lock.Migrations[0].Filepath != expectedFilepath {
		t.Errorf("Unexpected Filepath in lock. Expected: %v, Got: %v", expectedFilepath, lock.Migrations[0].Filepath)
	}

	if lock.Migrations[0].Timestamp != expectedTimestamp {
		t.Errorf("Unexpected Timestamp in lock. Expected: %v, Got: %v", expectedTimestamp, lock.Migrations[0].Timestamp.Format(time.RFC3339))
	}

	if lock.Migrations[0].Sum != expectedSum {
		t.Errorf("Unexpected Sum in lock. Expected: %v, Got: %v", expectedSum, lock.Migrations[0].Sum)
	}
}

func TestLoadLockFileFailure(t *testing.T) {
	_ = getTestDB()
	_, err := loadLockFile(testLockFailure)
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestGetDirFiles(t *testing.T) {
	dir := testPath("start")

	files, err := getDirFiles(dir)
	if err != nil {
		t.Fatalf("Error in getDirFiles: %v", err)
	}

	if files[0].Name() != "00-create-tables.sql" && files[1].Name() != "01-insert-records.sql" {
		t.Fatalf("invalid file list: %v", files)
	}
}

func TestExtractNumber(t *testing.T) {
	s := "123file.txt"
	regex := regexp.MustCompile(`^\d+`)

	num, isNum := extractNumber(s, regex)

	if !isNum || num != 123 {
		t.Errorf("Unexpected result from extractNumber. Expected: 123, Got: %d", num)
	}
}

func TestLockHasFile(t *testing.T) {
	lock := lock{
		Migrations: []Result{
			{
				Filepath:  "/test1.test",
				Timestamp: time.Now(),
				Sum:       "abcdef",
			},
		},
	}

	filepath := "/test1.test"
	hasFile := lockHasFile(lock, filepath)

	if !hasFile {
		t.Errorf("Expected lock to have file %s, but it does not.", filepath)
	}

	nonExistentFilepath := "/nonexistent.test"
	hasNonExistentFile := lockHasFile(lock, nonExistentFilepath)

	if hasNonExistentFile {
		t.Errorf("Expected lock to not have file %s, but it does.", nonExistentFilepath)
	}
}

func TestRun(t *testing.T) {
	db := getTestDB()
	lockFilePath := testPath(DefaultLockFile)
	_ = os.Remove(lockFilePath)

	config, err := loadConfigFile(testConfigStart)
	if err != nil {
		t.Fatalf("Error loading config file: %v", err)
	}

	results, err := Run(db, config)
	PrintOutput(results, err)
	if err != nil {
		t.Fatalf("Error migrating: %v", err)
	}

	config, err = loadConfigFile(testConfigIter)
	if err != nil {
		t.Fatalf("Error loading config file: %v", err)
	}

	results, err = Run(db, config)
	PrintOutput(results, err)
	if err != nil {
		t.Fatalf("Error migrating: %v", err)
	}

	if len(results) != 4 {
		t.Fatalf("invalid result set should be 4")
	}

	PrintOutput(Run(db, config))
}

func TestPrintOutputErr(t *testing.T) {
	PrintOutput(nil, errors.New("test error"))
}
