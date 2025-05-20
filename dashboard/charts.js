document.addEventListener("DOMContentLoaded", () => { fetch("../export/export.csv") .then(res => res.text()) .then(csv => parseAndRender(csv)); });

function parseAndRender(csv) { const rows = csv.trim().split("\n").slice(1); const tagCounts = {}; const dailyPosts = {}; const sentimentMap = { Positive: 0, Negative: 0, Neutral: 0 };

for (const row of rows) { const [title, url, fetched, tags, sentiment] = row.split(/,(?=(?:[^"]"[^"]")[^"]$)/); const date = new Date(fetched).toISOString().split("T")[0]; (dailyPosts[date] = (dailyPosts[date] || 0) + 1); sentimentMap[sentiment.trim()] = (sentimentMap[sentiment.trim()] || 0) + 1; tags.replace(/"/g, '').split(";").forEach(t => { if (t) tagCounts[t] = (tagCounts[t] || 0) + 1; }); }

const ctx1 = document.getElementById("sentimentChart").getContext("2d"); new Chart(ctx1, { type: "pie", data: { labels: Object.keys(sentimentMap), datasets: [{ data: Object.values(sentimentMap), backgroundColor: ["#4caf50", "#f44336", "#9e9e9e"] }] } });

const ctx2 = document.getElementById("tagChart").getContext("2d"); new Chart(ctx2, { type: "bar", data: { labels: Object.keys(tagCounts), datasets: [{ label: "Tag Count", data: Object.values(tagCounts), backgroundColor: "#42a5f5" }] } });

const ctx3 = document.getElementById("timelineChart").getContext("2d"); new Chart(ctx3, { type: "line", data: { labels: Object.keys(dailyPosts), datasets: [{ label: "Posts per Day", data: Object.values(dailyPosts), fill: true, backgroundColor: "rgba(255,193,7,0.3)", borderColor: "#ffc107" }] } }); }


