package model

type contextKey string

const (
	ContextAppName  contextKey = "appName"
	ContextAppVersion contextKey = "appVersion"
	ContextAppAuthor contextKey = "appAuthor"
	ContextConfigFile contextKey = "configFile"
	ContextPrintersFile contextKey = "printersFile"
	ContextAPIURL contextKey = "apiURL"
	ContextWSURL contextKey = "wsURL"
)