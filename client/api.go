// Copyright (c) 2018, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/globalsign/mgo/bson"
	"github.com/golang/glog"
	jsonresp "github.com/sylabs/json-resp"
)

// getEntity returns the specified entity
func (c *Client) getEntity(entityRef string) (*Entity, bool, error) {
	url := "/v1/entities/" + entityRef
	entJSON, found, err := c.apiGet(url)
	if err != nil {
		return nil, false, err
	}
	if !found {
		return nil, false, nil
	}
	var res EntityResponse
	if err := json.Unmarshal(entJSON, &res); err != nil {
		return nil, false, fmt.Errorf("error decoding entity: %v", err)
	}
	return &res.Data, found, nil
}

// getCollection returns the specified collection
func (c *Client) getCollection(collectionRef string) (*Collection, bool, error) {
	url := "/v1/collections/" + collectionRef
	colJSON, found, err := c.apiGet(url)
	if err != nil {
		return nil, false, err
	}
	if !found {
		return nil, false, nil
	}
	var res CollectionResponse
	if err := json.Unmarshal(colJSON, &res); err != nil {
		return nil, false, fmt.Errorf("error decoding collection: %v", err)
	}
	return &res.Data, found, nil
}

// getContainer returns container by ref id
func (c *Client) getContainer(containerRef string) (*Container, bool, error) {
	url := "/v1/containers/" + containerRef
	conJSON, found, err := c.apiGet(url)
	if err != nil {
		return nil, false, err
	}
	if !found {
		return nil, false, nil
	}
	var res ContainerResponse
	if err := json.Unmarshal(conJSON, &res); err != nil {
		return nil, false, fmt.Errorf("error decoding container: %v", err)
	}
	return &res.Data, found, nil
}

// createEntity creates an entity (must be authorized)
func (c *Client) createEntity(name string) (*Entity, error) {
	e := Entity{
		Name:        name,
		Description: "No description",
	}
	entJSON, err := c.apiCreate("/v1/entities", e)
	if err != nil {
		return nil, err
	}
	var res EntityResponse
	if err := json.Unmarshal(entJSON, &res); err != nil {
		return nil, fmt.Errorf("error decoding entity: %v", err)
	}
	return &res.Data, nil
}

// createCollection creates a new collection
func (c *Client) createCollection(name string, entityID string) (*Collection, error) {
	newCollection := Collection{
		Name:        name,
		Description: "No description",
		Entity:      bson.ObjectIdHex(entityID).Hex(),
	}
	colJSON, err := c.apiCreate("/v1/collections", newCollection)
	if err != nil {
		return nil, err
	}
	var res CollectionResponse
	if err := json.Unmarshal(colJSON, &res); err != nil {
		return nil, fmt.Errorf("error decoding collection: %v", err)
	}
	return &res.Data, nil
}

// createContainer creates a container in the specified collection
func (c *Client) createContainer(name string, collectionID string) (*Container, error) {
	newContainer := Container{
		Name:        name,
		Description: "No description",
		Collection:  bson.ObjectIdHex(collectionID).Hex(),
	}
	conJSON, err := c.apiCreate("/v1/containers", newContainer)
	if err != nil {
		return nil, err
	}
	var res ContainerResponse
	if err := json.Unmarshal(conJSON, &res); err != nil {
		return nil, fmt.Errorf("error decoding container: %v", err)
	}
	return &res.Data, nil
}

// createImage creates a new image
func (c *Client) createImage(hash string, containerID string, description string) (*Image, error) {
	i := Image{
		Hash:        hash,
		Description: description,
		Container:   bson.ObjectIdHex(containerID).Hex(),
	}
	imgJSON, err := c.apiCreate("/v1/images", i)
	if err != nil {
		return nil, err
	}
	var res ImageResponse
	if err := json.Unmarshal(imgJSON, &res); err != nil {
		return nil, fmt.Errorf("error decoding image: %v", err)
	}
	return &res.Data, nil
}

// setTags applies tags to the specified container
func (c *Client) setTags(containerID, imageID string, tags []string) error {
	// Get existing tags, so we know which will be replaced
	existingTags, err := c.getTags(containerID)
	if err != nil {
		return err
	}

	for _, tag := range tags {
		glog.Infof("Setting tag %s", tag)

		if _, ok := existingTags[tag]; ok {
			glog.Warningf("%s replaces an existing tag", tag)
		}

		imgTag := ImageTag{
			tag,
			bson.ObjectIdHex(imageID).Hex(),
		}
		err := c.setTag(containerID, imgTag)
		if err != nil {
			return err
		}
	}
	return nil
}

// Search searches library by name, returns any matching collections,
// containers, entities, or images.
func (c *Client) Search(value string) (*SearchResults, error) {
	url := fmt.Sprintf("/v1/search?value=%s", url.QueryEscape(value))

	resJSON, _, err := c.apiGet(url)
	if err != nil {
		return nil, err
	}

	var res SearchResponse
	if err := json.Unmarshal(resJSON, &res); err != nil {
		return nil, fmt.Errorf("error decoding results: %v", err)
	}

	return &res.Data, nil
}

func (c *Client) apiCreate(url string, o interface{}) (objJSON []byte, err error) {
	glog.V(2).Infof("apiCreate calling %s", url)
	s, err := json.Marshal(o)
	if err != nil {
		return []byte{}, fmt.Errorf("error encoding object to JSON:\n\t%v", err)
	}
	req, err := c.newRequest("POST", url, "", bytes.NewBuffer(s))
	if err != nil {
		return []byte{}, fmt.Errorf("error creating POST request:\n\t%v", err)
	}

	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return []byte{}, fmt.Errorf("error making request to server:\n\t%v", err)
	}
	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusCreated {
		err := jsonresp.ReadError(res.Body)
		if err != nil {
			return []byte{}, fmt.Errorf("creation did not succeed: %v", err)
		}
		return []byte{}, fmt.Errorf("creation did not succeed: http status code: %d", res.StatusCode)
	}
	objJSON, err = ioutil.ReadAll(res.Body)
	if err != nil {
		return []byte{}, fmt.Errorf("error reading response from server:\n\t%v", err)
	}
	return objJSON, nil
}

func (c *Client) apiGet(path string) (objJSON []byte, found bool, err error) {
	glog.V(2).Infof("apiGet calling %s", path)

	// split url containing query into component pieces (path and raw query)
	u, err := url.Parse(path)
	if err != nil {
		return []byte{}, false, fmt.Errorf("error parsing url:\n\t%v", err)
	}

	req, err := c.newRequest(http.MethodGet, u.Path, u.RawQuery, nil)
	if err != nil {
		return []byte{}, false, fmt.Errorf("error creating request to server:\n\t%v", err)
	}
	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return []byte{}, false, fmt.Errorf("error making request to server:\n\t%v", err)
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound {
		return []byte{}, false, nil
	}
	if res.StatusCode == http.StatusOK {
		objJSON, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return []byte{}, false, fmt.Errorf("error reading response from server:\n\t%v", err)
		}
		return objJSON, true, nil
	}
	// Not OK, not 404.... error
	err = jsonresp.ReadError(res.Body)
	if err != nil {
		return []byte{}, false, fmt.Errorf("get did not succeed: %v", err)
	}
	return []byte{}, false, fmt.Errorf("error reading response from server")
}

// getTags returns a tag map for the specified containerID
func (c *Client) getTags(containerID string) (TagMap, error) {
	url := fmt.Sprintf("/v1/tags/%s", containerID)
	glog.V(2).Infof("getTags calling %s", url)
	req, err := c.newRequest(http.MethodGet, url, "", nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request to server:\n\t%v", err)
	}
	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request to server:\n\t%v", err)
	}
	if res.StatusCode != http.StatusOK {
		err := jsonresp.ReadError(res.Body)
		if err != nil {
			return nil, fmt.Errorf("creation did not succeed: %v", err)
		}
		return nil, fmt.Errorf("unexpected http status code: %d", res.StatusCode)
	}
	var tagRes TagsResponse
	err = json.NewDecoder(res.Body).Decode(&tagRes)
	if err != nil {
		return nil, fmt.Errorf("error decoding tags: %v", err)
	}
	return tagRes.Data, nil
}

// setTag sets tag on specified containerID
func (c *Client) setTag(containerID string, t ImageTag) error {
	url := "/v1/tags/" + containerID
	glog.V(2).Infof("setTag calling %s", url)
	s, err := json.Marshal(t)
	if err != nil {
		return fmt.Errorf("error encoding object to JSON:\n\t%v", err)
	}
	req, err := c.newRequest("POST", url, "", bytes.NewBuffer(s))
	if err != nil {
		return fmt.Errorf("error creating POST request:\n\t%v", err)
	}
	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("error making request to server:\n\t%v", err)
	}
	if res.StatusCode != http.StatusOK {
		err := jsonresp.ReadError(res.Body)
		if err != nil {
			return fmt.Errorf("creation did not succeed: %v", err)
		}
		return fmt.Errorf("creation did not succeed: http status code: %d", res.StatusCode)
	}
	return nil
}

// GetImage returns the Image object if exists, otherwise returns error
func (c *Client) GetImage(imageRef string) (*Image, bool, error) {
	url := "/v1/images/" + imageRef
	imgJSON, found, err := c.apiGet(url)
	if err != nil {
		return nil, false, err
	}
	if !found {
		return nil, false, nil
	}
	var res ImageResponse
	if err := json.Unmarshal(imgJSON, &res); err != nil {
		return nil, false, fmt.Errorf("error decoding image: %v", err)
	}
	return &res.Data, found, nil
}
