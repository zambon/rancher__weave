package nameserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/miekg/dns"

	"github.com/weaveworks/weave/common"
	"github.com/weaveworks/weave/net/address"
	weaverouter "github.com/weaveworks/weave/router"
)

func badRequest(w http.ResponseWriter, err error) {
	http.Error(w, err.Error(), http.StatusBadRequest)
	common.Log.Warningf("[gossipdns]: %v", err)
}

func (n *Nameserver) HandleHTTP(router *mux.Router) {
	router.Methods("GET").Path("/domain").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, n.domain)
	})

	router.Methods("PUT").Path("/name/{container}/{ip}").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var (
			vars      = mux.Vars(r)
			container = vars["container"]
			ipStr     = vars["ip"]
			hostname  = r.FormValue("fqdn")
			ip, err   = address.ParseIP(ipStr)
		)
		if err != nil {
			badRequest(w, err)
			return
		}

		if err := n.AddEntry(dns.Fqdn(hostname), container, n.ourName, ip); err != nil {
			badRequest(w, fmt.Errorf("Unable to add entry: %v", err))
			return
		}

		if r.FormValue("check-alive") == "true" && n.docker.IsContainerNotRunning(container) {
			common.Log.Infof("[gossipdns] '%s' is not running: removing", container)
			if err := n.Delete(hostname, container, ipStr, ip); err != nil {
				common.Log.Infof("[gossipdns] failed to remove: %v", err)
			}
		}

		w.WriteHeader(204)
	})

	deleteHandler := func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)

		hostname := r.FormValue("fqdn")
		if hostname == "" {
			hostname = "*"
		} else {
			hostname = dns.Fqdn(hostname)
		}

		container, ok := vars["container"]
		if !ok {
			container = "*"
		}

		ipStr, ok := vars["ip"]
		ip, err := address.ParseIP(ipStr)
		if ok && err != nil {
			badRequest(w, err)
			return
		} else if !ok {
			ipStr = "*"
		}

		if err := n.Delete(hostname, container, ipStr, ip); err != nil {
			badRequest(w, fmt.Errorf("Unable to delete entries: %v", err))
			return
		}
		w.WriteHeader(204)
	}
	router.Methods("DELETE").Path("/name/{container}/{ip}").HandlerFunc(deleteHandler)
	router.Methods("DELETE").Path("/name/{container}").HandlerFunc(deleteHandler)
	router.Methods("DELETE").Path("/name").HandlerFunc(deleteHandler)

	router.Methods("GET").Path("/quarantine").Headers("Accept", "application/json").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		qs := n.Quarantines.List()

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(&qs); err != nil {
			badRequest(w, fmt.Errorf("Unable to serialise: %v", err))
		}
	})

	router.Methods("GET").Path("/quarantine").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "%-16s %-12s %-17s %s\n", "ID", "Container", "Peer", "Duration")
		for _, q := range n.Quarantines.List() {
			containerid := q.ContainerID
			if len(containerid) > 12 {
				containerid = containerid[:12]
			}
			peer := q.Peer.String()
			if q.Peer == weaverouter.UnknownPeerName {
				peer = ""
			}
			fmt.Fprintf(w, "%16s %12s %17s %s\n", q.ID, containerid, peer, time.Unix(q.ValidUntil, 0).String())
		}
	})

	router.Methods("POST").Path("/quarantine").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var (
			containerid = r.FormValue("containerid")
			peernameStr = r.FormValue("peer")
			durationStr = r.FormValue("duration")
		)

		peername, err := weaverouter.PeerNameFromString(peernameStr)
		if peernameStr != "" && err != nil {
			badRequest(w, fmt.Errorf("Cannot parse %s: %v", peernameStr, err))
		} else if peernameStr == "" {
			peername = weaverouter.UnknownPeerName
		}

		duration, err := time.ParseDuration(durationStr)
		if err != nil {
			badRequest(w, fmt.Errorf("Cannot parse %s: %v", durationStr, err))
		}

		id, err := n.Quarantines.Add(containerid, peername, duration)
		if err != nil {
			badRequest(w, fmt.Errorf("Unable to add quarantine: %v", err))
		}
		fmt.Fprintf(w, "%s\n", id)
	})

	router.Methods("DELETE").Path("/quarantine/{id}").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var (
			vars  = mux.Vars(r)
			ident = vars["id"]
		)
		if err := n.Quarantines.Delete(ident); err != nil {
			badRequest(w, fmt.Errorf("Unable to delete quarantine: %v", err))
		}
	})
}
