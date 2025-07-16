package json

type Transaction struct {
	Attributes  string `json:"attributes"`
	Fee         int    `json:"fee"`
	Hash        string `json:"hash"`
	Nonce       int    `json:"nonce"`
	PayloadData string `json:"payloadData"`
	Programs    []struct {
		Code      string `json:"code"`
		Parameter string `json:"parameter"`
	} `json:"programs"`
	Size   int    `json:"size"`
	TxType string `json:"txType"`
}

type Transactions []struct {
	Attributes  string `json:"attributes"`
	Fee         int    `json:"fee"`
	Hash        string `json:"hash"`
	Nonce       int    `json:"nonce"`
	PayloadData string `json:"payloadData"`
	Programs    []struct {
		Code      string `json:"code"`
		Parameter string `json:"parameter"`
	} `json:"programs"`
	Size   int    `json:"size"`
	TxType string `json:"txType"`
}

/*type Transfer struct {
	Data struct {
		Sigchain    any `json:"sigchain"`
		Transaction struct {
			Hash            string `json:"Hash"`
			Height          int    `json:"Height"`
			HeightIdxUnion  string `json:"HeightIdxUnion"`
			TxType          int    `json:"TxType"`
			Attributes      string `json:"Attributes"`
			Fee             int    `json:"Fee"`
			Nonce           int    `json:"Nonce"`
			AssetID         string `json:"AssetId"`
			UTXOInputCount  int    `json:"UTXOInputCount"`
			UTXOOutputCount int    `json:"UTXOOutputCount"`
			Timestamp       string `json:"Timestamp"`
			ParseStatus     int    `json:"ParseStatus"`
		} `json:"transaction"`
		Transfer []struct {
			Hash        string `json:"Hash"`
			Height      int    `json:"Height"`
			HeightTxIdx string `json:"HeightTxIdx"`
			FromAddr    string `json:"FromAddr"`
			ToAddr      string `json:"ToAddr"`
			AssetID     string `json:"AssetId"`
			Value       string `json:"Value"`
			Fee         string `json:"Fee"`
			Timestamp   string `json:"Timestamp"`
		} `json:"transfer"`
	} `json:"Data"`
	Timestamp  string `json:"Timestamp"`
	APIVersion string `json:"ApiVersion"`
}
*/
