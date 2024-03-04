package biscepter

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPerformSingleHealthcheck(t *testing.T) {
	t.Run("Test HTTP healthcheck", func(t *testing.T) {
		t.Run("Unhealthy endpoint fails", func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(500)
			}))
			defer server.Close()

			check := Healthcheck{
				Port:      1337,
				CheckType: HttpGet200,
			}

			// [0] := "http", [1]: //127.0.0.1, [2]: actual port
			port, err := strconv.Atoi(strings.Split(server.URL, ":")[2])
			assert.Nil(t, err, "couldn't get port of testing server")

			ok, _ := check.performSingleHealthcheck(map[int]int{
				1337: port,
			})

			assert.False(t, ok, "Unhealthy endpoint resulted in successful healthcheck")
		})
		t.Run("Healthy endpoint succeeds", func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(200)
			}))
			defer server.Close()

			check := Healthcheck{
				Port:      1337,
				CheckType: HttpGet200,
			}

			// [0] := "http", [1]: //127.0.0.1, [2]: actual port
			port, err := strconv.Atoi(strings.Split(server.URL, ":")[2])
			assert.Nil(t, err, "couldn't get port of testing server")

			ok, err := check.performSingleHealthcheck(map[int]int{
				1337: port,
			})

			assert.True(t, ok, "Healthy endpoint resulted in failed healthcheck")
			assert.Nil(t, err, "Healthy endpoint resulted in an error being returned")
		})
	})
	t.Run("Test script healthcheck", func(t *testing.T) {
		t.Run("Unhealthy endpoint fails", func(t *testing.T) {
			check := Healthcheck{
				CheckType: Script,
				Data:      "exit 1",
			}

			ok, _ := check.performSingleHealthcheck(map[int]int{})

			assert.False(t, ok, "Unhealthy endpoint resulted in successful healthcheck")
		})
		t.Run("Healthy endpoint succeeds", func(t *testing.T) {
			check := Healthcheck{
				CheckType: Script,
				Data:      "exit 0",
			}

			ok, _ := check.performSingleHealthcheck(map[int]int{})

			assert.True(t, ok, "Healthy endpoint resulted in failed healthcheck")
		})
		t.Run("Port environment variable gets substituted correctly", func(t *testing.T) {
			check := Healthcheck{
				Port:      1337,
				CheckType: Script,
				Data:      "if [ $PORT1337 -eq 42 ]; then exit 0; fi; exit 1",
			}

			ok, _ := check.performSingleHealthcheck(map[int]int{
				1337: 42,
			})

			assert.True(t, ok, "Healthy endpoint resulted in unsuccessful healthcheck")
		})
	})
}
