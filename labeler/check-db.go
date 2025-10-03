package main

import (
  "fmt"
  "io"
  "net/http"
  "os"
)

func main() {
  url := os.Getenv("SUPABASE_URL") // e.g. https://abcd.supabase.co
  key := os.Getenv("SUPABASE_KEY")
  if url == "" || key == "" {
    fmt.Println("Set SUPABASE_URL and SUPABASE_KEY env vars")
    return
  }
  req, _ := http.NewRequest("GET", url+"/rest/v1/?select=", nil)
  req.Header.Set("apikey", key)
  req.Header.Set("Authorization", "Bearer "+key)
  client := &http.Client{}
  resp, err := client.Do(req)
  if err != nil {
    fmt.Println("request error:", err)
    return
  }
  defer resp.Body.Close()
  body, _ := io.ReadAll(resp.Body)
  fmt.Println("status:", resp.Status)
  fmt.Println("body:", string(body))
}