async function main() {
  const queryInput =
    input && typeof input.query === "string" ? input.query : "";
  const query = queryInput.trim() || "tops for women";
  const querySlug = query.trim().toLowerCase().replace(/\s+/g, "-");
  const queryEncoded = encodeURIComponent(query);

  const searches = [
    {
      retailer: "amazon",
      url: `https://www.amazon.in/s?k=${queryEncoded}`,
      loadSelectors: [
        ".s-main-slot .s-result-item[data-asin]",
        "#noResultsTitle",
      ],
    },
    {
      retailer: "myntra",
      url: `https://www.myntra.com/${querySlug}?rawQuery=${queryEncoded}`,
      loadSelectors: [".results-base", ".product-base", ".no-results"],
    },
    {
      retailer: "ajio",
      url: `https://www.ajio.com/search/?text=${queryEncoded}`,
      loadSelectors: [".items", ".item", ".no-results-msg"],
    },
  ];

  const results = [];

  for (const search of searches) {
    try {
      await navigate(search.url, { referer: "https://www.google.com/" });
      await wait_any(search.loadSelectors, { timeout: 15000 });

      const products = parse(); // array of products for this retailer

      if (!Array.isArray(products) || products.length === 0) {
        results.push({
          retailer: search.retailer,
          url: search.url,
          success: true,
          count: 0,
        });
        continue;
      }

      for (const product of products) {
        collect(product);
      }

      results.push({
        retailer: search.retailer,
        url: search.url,
        success: true,
        count: products.length,
      });
    } catch (error) {
      results.push({
        retailer: search.retailer,
        url: search.url,
        success: false,
        error: error.message,
      });
    }
  }

  return { query, results };
}

return main();
