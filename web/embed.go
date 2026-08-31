package web

import "embed"

//go:embed index.html admin/* supervisor/* user/* shared/*
var UIFS embed.FS
