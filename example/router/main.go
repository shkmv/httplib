package main

import (
    "fmt"
    "log"
    "net/http"
    "time"

    "github.com/shkmv/httplib/router"
    "github.com/shkmv/httplib/router/ctxutil"
    rmid "github.com/shkmv/httplib/router/middleware"
)

func main() {
	r := router.New()

    // Essential middlewares
    r.Use(
        rmid.RealIP(),
        rmid.RequestID(),
        rmid.Logger(nil),
        rmid.Recoverer(nil),
        rmid.NoCache(),
        rmid.Timeout(5*time.Second, "request timeout"),
        rmid.CORS(),
    )

	r.GetFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "pong")
	})

	r.Route("/api", func(api *router.Router) {
        api.GetFunc("/users", func(w http.ResponseWriter, r *http.Request) {
            ip := ctxutil.GetRealIP(r.Context())
            reqID := ctxutil.GetReqID(r.Context())
            fmt.Fprintf(w, "users list (ip=%s req_id=%s)", ip, reqID)
        })

		// Mount nested subrouter
		admin := router.New()
		admin.GetFunc("/dashboard", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, "admin dashboard")
		})
		api.Mount("/admin", admin)
	})

	log.Println("listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", r))
}
