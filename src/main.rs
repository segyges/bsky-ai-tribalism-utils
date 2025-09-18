use reqwest;
use tokio;

struct ConstellationClient {
	root_uri: String,
}

impl ConstellationClient {
    // Basic method syntax - this is how you add methods to structs
    fn get_root_uri(&self) -> &str {
        &self.root_uri
    }

		async fn get_links(
			&self, target: &str,
			collection: &str,
			path: &str
		) -> Result<String, Box<dyn std::error::Error>> {
			let params = [
				("target", target),
				("collection", collection),
				("path", path), 
			];

			let mut url = reqwest::Url::parse_with_params(
				&self.get_root_uri(),
				&params,
			)?;
			url.set_path("/links");
			let body: String = reqwest::get(url)
				.await?
				.text()
				.await?;

			Ok(body)
		}
}

#[tokio::main]
async fn main() {
		let _list_of_anti_lists: &str = include_str!("./anti-ai-lists.txt");
		let cli = ConstellationClient { root_uri: "https://constellation.microcosm.blue/".to_string() };

    let res = cli.get_links(
			"at://did:plc:ltqxsidueahnlsandk4u4nwl/app.bsky.graph.list/3kdsk7c6rp22w",
			"app.bsky.graph.listblock",
		".subject").await;
    match res {
        Ok(body) => println!("{}", body),
        Err(e) => println!("Error: {}", e),
    }
}