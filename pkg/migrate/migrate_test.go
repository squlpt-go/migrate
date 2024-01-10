package migrate

import (
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
	"os"
	"path"
	"reflect"
	"regexp"
	"testing"
	"time"
)

func testDir() string {
	dirname, err := CurrentDirname()
	if err != nil {
		panic(err)
	}
	return dirname + "/../../test/"
}

var (
	testConfigStart   = testDir() + "config-start.json"
	testConfigIter    = testDir() + "config-iter.json"
	testConfigFailure = testDir() + "config-failure.json"
	testLock          = testDir() + "test-lock.json"
	testLockFailure   = testDir() + "test-lock-failure.json"
)

func getTestDB() *sql.DB {
	dirname, err := CurrentDirname()
	if err != nil {
		panic(err)
	}
	envFilePath := dirname + "/../../.migrate.env"
	err = godotenv.Load(envFilePath)
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
	_ = getTestDB()
	config, err := LoadConfigFile(testConfigIter)
	if err != nil {
		t.Fatalf("Error loading config file: %v", err)
	}

	expectedDirs := []string{path.Join(testDir(), "start/*.sql"), path.Join(testDir(), "iter/*.sql")}

	if !reflect.DeepEqual(config.Paths, expectedDirs) {
		t.Errorf("Unexpected Dirs in config. Expected: %v, Got: %v", expectedDirs, config.Paths)
	}
}

func TestLoadConfigFileFails(t *testing.T) {
	_ = getTestDB()
	_, err := LoadConfigFile(testConfigFailure)
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
	dir := testDir() + "start"

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
	lock := Lock{
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

func TestMigrate(t *testing.T) {
	db := getTestDB()
	lockFilePath := path.Join(testDir(), DefaultLockFile)
	_ = os.Remove(lockFilePath)

	config, err := LoadConfigFile(testConfigStart)
	if err != nil {
		t.Fatalf("Error loading config file: %v", err)
	}

	results, err := Migrate(db, config)
	if err != nil {
		t.Fatalf("Error migrating: %v", err)
	}

	config, err = LoadConfigFile(testConfigIter)
	if err != nil {
		t.Fatalf("Error loading config file: %v", err)
	}

	results, err = Migrate(db, config)
	if err != nil {
		t.Fatalf("Error migrating: %v", err)
	}

	t.Log(results)
}
