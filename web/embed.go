package web

import "embed"

// UIFS is the embedded production console tree (U.2 shells).
//
//go:embed index.html admin/* supervisor/* chat/* shared/*
var UIFS embed.FS
