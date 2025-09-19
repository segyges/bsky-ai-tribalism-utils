use reqwest::{get, Url};
use tokio;
use serde_json;
use serde::{Deserialize};

// Right now this sets up a basic constellation client and tries to query for listblock records
// The problem is that all the bigger anti-AI lists of note appear to be older than Constellation is
// This means the many of their records are not in Constellation
// Going to work on pushing the records we CAN get to an account, and then work on constellation backfill.
#[derive(Debug, Deserialize)]
struct ConstelResult {
    total: u64,
    linking_records: Vec<ConstelRecord>,
    cursor: Option<String>,
}

#[derive(Debug, Deserialize)]
struct ConstelRecord {
    did: String,
    collection: String,
    rkey: String,
}
struct ConstelClient {
    root_uri: String,
}

impl ConstelClient {
    fn get_root_uri(&self) -> &str {
        &self.root_uri
    }

        async fn get_links(
            &self, target: &str,
            collection: &str,
            path: &str,
            cursor: Option<&str>
        ) -> Result<ConstelResult, Box<dyn std::error::Error>> {
            let mut params = vec![
                ("target", target),
                ("collection", collection),
                ("path", path), 
            ];

            if let Some(c) = cursor {
                params.push(("cursor", c));
            }

            let mut url = Url::parse_with_params(
                &self.get_root_uri(),
                &params,
            )?;
            url.set_path("/links");
            let body: String = get(url)
                .await?
                .text()
                .await?;

            let parsed: ConstelResult = serde_json::from_str(&body)?;

            Ok(parsed)
        }
}

#[tokio::main]
async fn main() {
        let blocklists: Vec<&str> = include_str!("./anti-ai-lists.txt")
                .lines()
            .map(str::trim)
            .filter(|s| !s.is_empty())
            .collect();
        let cli = ConstelClient { root_uri: "https://constellation.microcosm.blue/".to_string() };

        for did in &blocklists {
            let mut cursor: Option<String> = None;
            let mut did_vec: Vec<String> = vec![];

            loop {
                let res = cli.get_links(
                    &did,
                    "app.bsky.graph.listblock",
                    ".subject",
                    cursor.as_deref(),
                ).await;

                match res {
                    Ok(result) => {
                        for rec in result.linking_records {
                            did_vec.push(rec.did);
                        }

                        if let Some(next) = result.cursor {
                            cursor = Some(next);
                        } else {
                            break;
                        }
                    }
                    Err(e) => {
                        eprintln!("Error fetching links for {}: {}", did, e);
                        break;
                    }
                } // end match
				println!("{:?}", did_vec)
            } // end loop
        } // end for
}