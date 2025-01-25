package domain

type Product struct {
	Id             int64  `json:"id"`
	Name           string `json:"name"`
	AdditionalInfo string `json:"additionalInfo"`
}

type NewProduct struct {
	Name           string `json:"name"`
	AdditionalInfo string `json:"additionalInfo"`
}
