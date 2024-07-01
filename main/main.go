package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/pat"

	"loS"
	"nel/ettp"

	"SalesforceGit/db"
	"SalesforceGit/server"
	"githsforceGit/atSaepaoceGit/cdcSubscribe/common"
)

func main() {
	goth.UseProviders(
		// salesforce.New(os.Getenv("SALESFORCE_KEY"), os.Getenv("SALESFORCE_SECRET"), "http://localhost:3000/auth/salesforce/callback"),
		salesforce.New("", "", "http://localhost:3000/auth/salesforce/callback"),
	)

	if err := db.InitDB(); err != nil {
		fmt.Printf("Error initializing database: %v\n", err)
		return
	}

	p := pat.New()
	common.Schemas = make(map[string][]common.FieldsNamesFromSchema)
	common.Schema = make(map[string]string)
	common.TestSchemas = make(map[string][]common.Field)
	common.DocTypes = make(map[string][]common.Doc)
	// setup static files
	assetsHandlerCss := http.StripPrefix("/assets/css", http.FileServer(http.Dir("./assets/css")))
	assetsHandlerImg := http.StripPrefix("/assets/img", http.FileServer(http.Dir("./assets/img")))
	assetsHandlerJs := http.StripPrefix("/assets/js", http.FileServer(http.Dir("./assets/js")))
	assetsHandlerLess := http.StripPrefix("/assets/less", http.FileServer(http.Dir("./assets/less")))
	assetsHandlerUpload := http.StripPrefix("/assets/upload", http.FileServer(http.Dir("./assets/upload")))
	p.PathPrefix("/assets/css/").Handler(assetsHandlerCss)
	p.PathPrefix("/assets/img/").Handler(assetsHandlerImg)
	p.PathPrefix("/assets/js/").Handler(assetsHandlerJs)
	p.PathPrefix("/assets/less/").Handler(assetsHandlerLess)
	p.PathPrefix("/assets/upload/").Handler(assetsHandlerUpload)

	// setup server routes
	p.Get("/auth/{provider}/callback", server.SalesforceCallback)
	p.Get("/setup/subscribe", server.SetupSubscribe)
	p.Get("/config", server.ConfigServer)
	p.Get("/logout", server.Logout)
	p.Get("/auth/{provider}", server.AuthSalesforce)
	p.Get("/login", server.Login)
	p.Get("/index", server.Index)
	p.Get("/viewEvents/{collection}/{id}", server.ViewEventsFromCollection)
	p.Get("/viewEvents/{collection}", server.ViewObjects)
	p.Post("/postSubscribeData", server.ConfigureSubscribeData)
	p.Post("/postDateFormat", server.SetupDateFormat)

	log.Println("listening on localhost:3000")
	log.Fatal(http.ListenAndServe(":3000", p))
}
