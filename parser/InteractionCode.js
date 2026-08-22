async function main() {
  // ------------------------------------------------------------
  // Read query supplied by API / Studio input
  //
  // Supported:
  //   { "query": "nike shoes" }
  //   { "search_query": "nike shoes" }
  //   { "q": "nike shoes" }
  // ------------------------------------------------------------

  const rawQuery =
    input && typeof input.query === "string"
      ? input.query
      : input && typeof input.search_query === "string"
        ? input.search_query
        : input && typeof input.q === "string"
          ? input.q
          : "";

  const query = rawQuery.trim();

  // Do NOT silently search for another product when API input is missing.
  if (!query) {
    bad_input(
      'Missing required input. Send JSON such as {"query":"nike shoes"}',
    );

    return {
      success: false,
      error: "Missing required query",
    };
  }

  const querySlug = query
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");

  const queryEncoded = encodeURIComponent(query);

  // ------------------------------------------------------------
  // Retailer search URLs
  // ------------------------------------------------------------

  const searches = [
    {
      retailer: "amazon",

      url: `https://www.amazon.in/s?k=${queryEncoded}`,

      loadSelectors: [
        ".s-main-slot .s-result-item[data-asin]",
        "[data-asin]",
        "#search",
        "#noResultsTitle",
      ],
    },

    {
      retailer: "myntra",

      url: `https://www.myntra.com/${querySlug}?rawQuery=${queryEncoded}`,

      loadSelectors: [
        ".results-base",
        ".product-base",
        "[class*='product-base']",
        "[class*='product-card']",
        ".results-showcase",
        ".no-results",
      ],
    },

    {
      retailer: "ajio",

      url: `https://www.ajio.com/search/?text=${queryEncoded}`,

      loadSelectors: [
        ".items",
        ".item",
        "[class*='item']",
        "[class*='product']",
        "[class*='card']",
        ".no-results-msg",
      ],
    },
  ];

  const results = [];

  // ------------------------------------------------------------
  // Search each retailer
  // ------------------------------------------------------------

  for (const search of searches) {
    try {
      console.log(`Searching ${search.retailer} for query: ${query}`);

      await navigate(search.url, {
        referer: "https://www.google.com/",
        wait_until: "domcontentloaded",
      });

      // Give dynamically rendered retail pages a little time.
      try {
        await wait_any(search.loadSelectors, {
          timeout: 20000,
        });
      } catch (waitError) {
        // Some retailers don't expose the expected selector
        // immediately. Continue to parsing instead of failing
        // the entire retailer.
        console.log(
          `${search.retailer}: selector wait timed out, attempting parse anyway`,
        );
      }

      // Additional rendering time for React/Vue-style pages.
      try {
        await wait_page_idle({ timeout: 5000 });
      } catch (idleError) {
        // Ignore idle timeout.
      }

      // --------------------------------------------------------
      // Parser automatically detects retailer using location.href
      // --------------------------------------------------------

      const products = parse();

      if (!Array.isArray(products)) {
        results.push({
          retailer: search.retailer,
          url: search.url,
          success: false,
          count: 0,
          error: "Parser did not return an array",
        });

        continue;
      }

      // --------------------------------------------------------
      // Collect products
      // --------------------------------------------------------

      let collected = 0;

      for (const product of products) {
        if (!product || !product.title) {
          continue;
        }

        collect(product);
        collected++;
      }

      results.push({
        retailer: search.retailer,
        url: search.url,
        success: true,
        count: collected,
      });
    } catch (error) {
      console.log(
        `${search.retailer} failed:`,
        error && error.message ? error.message : String(error),
      );

      results.push({
        retailer: search.retailer,
        url: search.url,
        success: false,
        count: 0,
        error: error && error.message ? error.message : String(error),
      });
    }
  }

  // ------------------------------------------------------------
  // API response
  // ------------------------------------------------------------

  return {
    success: true,
    query: query,
    results: results,
  };
}

return main();
