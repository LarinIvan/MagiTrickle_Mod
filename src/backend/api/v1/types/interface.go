package types

type InterfacesRes struct {
	Interfaces []InterfaceRes `json:"interfaces,omitempty"`
}

type InterfaceRes struct {
	ID     string `json:"id" example:"nwg0" swaggertype:"string"`
	Active bool   `json:"active" example:"true" swaggertype:"boolean"`
	IP     string `json:"ip" example:"192.168.1.1" swaggertype:"string"`
}
