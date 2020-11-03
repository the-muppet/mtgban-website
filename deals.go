package main

import (
	"net/http"
	"strconv"
)

func Deals(w http.ResponseWriter, r *http.Request) {
	sig := r.FormValue("sig")

	pageVars := genPageNav("Search", sig)

	if !DatabaseLoaded {
		pageVars.Title = "Great things are coming"
		pageVars.ErrorMessage = "Website is starting, please try again in a few minutes"

		render(w, "deals.html", pageVars)
		return
	}

	dealsParam, _ := GetParamFromSig(sig, "Search")
	canDeal, _ := strconv.ParseBool(dealsParam)
	if SigCheck && !canDeal {
		pageVars.Title = "This feature is BANned"
		pageVars.ErrorMessage = ErrMsgPlus
		pageVars.ShowPromo = true

		render(w, "deals.html", pageVars)
		return
	}

	render(w, "deals.html", pageVars)
}
