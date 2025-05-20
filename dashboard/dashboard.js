document.addEventListener("DOMContentLoaded", () => {
  fetch("../data/state.json")
    .then(res => res.json())
    .then(renderNews)
    .catch(err => console.error("Failed to load state.json:", err));
});

function renderNews(data) {
  const container = document.getElementById("news-container");
  container.innerHTML = "";

  const searchInput = document.getElementById("search");
  const sentimentFilter = document.getElementById("sentiment");

  searchInput.addEventListener("input", () => renderNews(data));
  sentimentFilter.addEventListener("change", () => renderNews(data));

  const entries = (data.articles || []).sort((a, b) => new Date(b.fetched) - new Date(a.fetched));
  const searchTerm = searchInput.value.toLowerCase();
  const sentiment = sentimentFilter.value;

  for (const article of entries) {
    if (searchTerm && !article.title.toLowerCase().includes(searchTerm) && !article.tags.join(" ").toLowerCase().includes(searchTerm)) {
      continue;
    }
    if (sentiment && article.sentiment !== sentiment) {
      continue;
    }

    const card = document.createElement("div");
    const tagClass = article.tags.length > 0 ? `card--${article.tags[0].toLowerCase()}` : "";
    card.className = `card ${tagClass}`;
    card.innerHTML = `
      <h3>${article.title}</h3>
      <p><strong>Tags:</strong> ${article.tags.join(", ")}</p>
      <p><strong>Sentiment:</strong> ${article.sentiment || "Unknown"}</p>
      <p><strong>Fetched:</strong> ${new Date(article.fetched).toLocaleString()}</p>
      <p><strong>Summary:</strong> ${article.summary || "No summary."}</p>
      <p><a href="${article.url}" target="_blank">Open Article</a></p>
      ${article.thread_url ? `<p><a href="${article.thread_url}" target="_blank">View Thread</a></p>` : ""}
    `;
    container.appendChild(card);
  }
}
