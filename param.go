// Copyright (c) Jeevanandam M. (https://github.com/jeevatkm)
// go-aah/aah source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package aah

import (
	"html/template"
	"io/ioutil"
	"net/http"

	"aahframework.org/ahttp.v0"
	"aahframework.org/log.v0"
)

const keyRequestParams = "RequestParams"

//‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾
// Params middleware
//___________________________________

// ParamsMiddleware parses the incoming HTTP request to collects request
// parameters (query string and payload) stores into controller. Query string
// parameters made available in render context.
func paramsMiddleware(c *Controller, m *Middleware) {
	req := c.Req.Raw

	if c.Req.Method != ahttp.MethodGet {
		contentType := c.Req.ContentType.Mime
		log.Debugf("request content type: %s", contentType)

		switch contentType {
		case ahttp.ContentTypeJSON.Mime, ahttp.ContentTypeXML.Mime:
			if payloadBytes, err := ioutil.ReadAll(req.Body); err == nil {
				c.Req.Payload = string(payloadBytes)
			} else {
				log.Errorf("unable to read request body for '%s': %s", contentType, err)
			}
		case ahttp.ContentTypeForm.Mime:
			if err := req.ParseForm(); err == nil {
				c.Req.Params.Form = req.Form
			} else {
				log.Errorf("unable to parse form: %s", err)
			}
		case ahttp.ContentTypeMultipartForm.Mime:
			if isMultipartEnabled {
				if err := req.ParseMultipartForm(appMultipartMaxMemory); err == nil {
					c.Req.Params.Form = req.MultipartForm.Value
					c.Req.Params.File = req.MultipartForm.File
				} else {
					log.Errorf("unable to parse multipart form: %s", err)
				}
			} else {
				log.Warn("multipart processing is disabled in aah.conf")
			}
		} // switch end

		// clean up
		defer func(r *http.Request) {
			if r.MultipartForm != nil {
				log.Debug("multipart form file clean up")
				if err := r.MultipartForm.RemoveAll(); err != nil {
					log.Error(err)
				}
			}
		}(req)
	}

	// All the request parameters made available to templates via funcs.
	c.AddViewArg(keyRequestParams, c.Req.Params)

	m.Next(c)

}

//‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾
// Template methods
//___________________________________

// tmplPathParam method returns Request Path Param value for the given key.
func tmplPathParam(viewArgs map[string]interface{}, key string) template.HTML {
	params := viewArgs[keyRequestParams].(*ahttp.Params)
	return template.HTML(params.PathValue(key))
}

// tmplFormParam method returns Request Form value for the given key.
func tmplFormParam(viewArgs map[string]interface{}, key string) template.HTML {
	params := viewArgs[keyRequestParams].(*ahttp.Params)
	return template.HTML(params.FormValue(key))
}

// tmplQueryParam method returns Request Query String value for the given key.
func tmplQueryParam(viewArgs map[string]interface{}, key string) template.HTML {
	params := viewArgs[keyRequestParams].(*ahttp.Params)
	return template.HTML(params.QueryValue(key))
}
