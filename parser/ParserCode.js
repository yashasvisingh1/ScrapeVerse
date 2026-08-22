// ============================================================
// RETAILER DETECTION
// ============================================================

function detectRetailer() {
  const href = location.href || "";

  if (href.includes("amazon.in")) {
    return "amazon";
  }

  if (href.includes("myntra.com")) {
    return "myntra";
  }

  if (href.includes("ajio.com")) {
    return "ajio";
  }

  return null;
}

// ============================================================
// INPUT QUERY
// ============================================================

function getInputQuery() {
  if (!input) {
    return null;
  }

  if (typeof input.query === "string" && input.query.trim()) {
    return input.query.trim();
  }

  if (typeof input.search_query === "string" && input.search_query.trim()) {
    return input.search_query.trim();
  }

  if (typeof input.q === "string" && input.q.trim()) {
    return input.q.trim();
  }

  return null;
}

// ============================================================
// TEXT HELPERS
// ============================================================

function cleanText(value) {
  if (value === null || value === undefined) {
    return "";
  }

  return String(value).replace(/\s+/g, " ").trim();
}

function firstText(el, selectors) {
  for (const selector of selectors) {
    const value = $(el).find(selector).first().text_sane();

    if (value) {
      return value;
    }
  }

  return "";
}

function firstAttr(el, selectors, attr) {
  for (const selector of selectors) {
    const value = $(el).find(selector).first().attr(attr);

    if (value) {
      return value;
    }
  }

  return null;
}

// ============================================================
// PRICE PARSER
//
// Important:
// Do NOT use:
//     text.replace(/[^\d.]/g, "")
//
// because it can turn malformed strings such as ".5"
// into 0.5.
//
// For INR product prices we want:
//   ₹699
//   ₹ 699
//   Rs. 699
//   Rs 1,299
//   1,299
//   ₹1,299.00
//
// into:
//   699
//   1299
//
// Values below ₹1 are rejected.
// ============================================================

function parsePrice(text) {
  if (!text) {
    return null;
  }

  let value = cleanText(text);

  if (!value) {
    return null;
  }

  // Remove common currency labels.
  value = value
    .replace(/₹/g, "")
    .replace(/INR/gi, "")
    .replace(/Rs\.?/gi, "")
    .trim();

  // Find an INR-like numeric amount.
  //
  // Supports:
  // 699
  // 1,299
  // 699.00
  // 1,299.00
  //
  const matches = value.match(/\d[\d,]*(?:\.\d{1,2})?/g);

  if (!matches || matches.length === 0) {
    return null;
  }

  for (const match of matches) {
    const normalized = match.replace(/,/g, "");

    const parsed = Number(normalized);

    if (!Number.isFinite(parsed)) {
      continue;
    }

    // Product prices below ₹1 are almost certainly
    // malformed extraction rather than a real INR price.
    if (parsed < 1) {
      continue;
    }

    // Return normal numeric INR value.
    return parsed;
  }

  return null;
}

// ============================================================
// RATING PARSER
// ============================================================

function parseRating(text) {
  if (!text) {
    return null;
  }

  const value = cleanText(text);

  // Find something like:
  // 4.5
  // 4.3
  // 3.9
  const match = value.match(/\b([0-5](?:\.\d)?)\b/);

  if (!match) {
    return null;
  }

  const rating = Number(match[1]);

  if (!Number.isFinite(rating)) {
    return null;
  }

  if (rating < 0 || rating > 5) {
    return null;
  }

  return rating;
}

// ============================================================
// URL HELPER
// ============================================================

function absoluteUrl(url) {
  if (!url) {
    return null;
  }

  try {
    return new URL(url, location.href).href;
  } catch (error) {
    return url;
  }
}

// ============================================================
// IMAGE HELPER
// ============================================================

function getImageUrl(el, selectors) {
  for (const selector of selectors) {
    const image = $(el).find(selector).first();

    if (!image || image.length === 0) {
      continue;
    }

    const src =
      image.attr("src") ||
      image.attr("data-src") ||
      image.attr("data-original") ||
      image.attr("data-lazy-src");

    if (src) {
      return absoluteUrl(src);
    }
  }

  return null;
}

// ============================================================
// AMAZON
// ============================================================

function parseAmazonResults() {
  const items = [];
  const seen = new Set();

  $(".s-main-slot .s-result-item[data-asin], [data-asin]").each((i, el) => {
    const asin = $(el).attr("data-asin");

    if (!asin || asin === "undefined") {
      return;
    }

    if (seen.has(asin)) {
      return;
    }

    seen.add(asin);

    const title = firstText(el, ["h2 span", "h2", "[data-cy='title-recipe']"]);

    if (!title) {
      return;
    }

    // ------------------------------
    // Current price
    // ------------------------------

    const priceWhole = $(el)
      .find(".a-price .a-price-whole")
      .first()
      .text()
      .replace(/[^\d]/g, "");

    const priceFraction = $(el)
      .find(".a-price .a-price-fraction")
      .first()
      .text()
      .replace(/[^\d]/g, "");

    let currentPrice = null;

    if (priceWhole) {
      const combined = priceFraction
        ? `${priceWhole}.${priceFraction}`
        : priceWhole;

      currentPrice = parsePrice(combined);
    }

    // Fallback
    if (currentPrice === null) {
      currentPrice = parsePrice(
        $(el).find(".a-price .a-offscreen").first().text(),
      );
    }

    // ------------------------------
    // Original price
    // ------------------------------

    const originalPrice = parsePrice(
      $(el).find(".a-price.a-text-price .a-offscreen").first().text(),
    );

    // ------------------------------
    // Rating
    // ------------------------------

    const rating = parseRating(
      firstText(el, [
        ".a-icon-star-small .a-icon-alt",
        ".a-icon-star .a-icon-alt",
        ".a-icon-alt",
      ]),
    );

    // ------------------------------
    // Reviews
    // ------------------------------

    const reviewText = firstText(el, [
      ".a-size-base.s-underline-text",
      "a[href*='#customerReviews']",
      "[data-csa-c-content-id='alf-customerReviews']",
    ]);

    const reviewNumbers = reviewText.match(/\d[\d,]*/);

    const reviewCount = reviewNumbers
      ? parseInt(reviewNumbers[0].replace(/,/g, ""), 10)
      : null;

    // ------------------------------
    // URL
    // ------------------------------

    const externalUrl = absoluteUrl($(el).find("h2 a").first().attr("href"));

    // ------------------------------
    // Image
    // ------------------------------

    const imageUrl = getImageUrl(el, ["img.s-image", "img"]);

    items.push({
      retailerName: "Amazon",
      query: getInputQuery(),

      productId: asin,

      title: cleanText(title),
      brand: null,

      currentPrice: currentPrice,
      originalPrice: originalPrice !== null ? originalPrice : currentPrice,

      currency: "INR",

      rating: rating,
      reviewCount: reviewCount,

      imageUrl: imageUrl,
      externalUrl: externalUrl,

      scrapedAt: new Date().toISOString(),
    });
  });

  return items;
}

// ============================================================
// MYNTRA
// ============================================================

function parseMyntraResults() {
  const items = [];
  const seen = new Set();

  const selectors = [".product-base", "[class*='product-base']"];

  let found = false;

  for (const selector of selectors) {
    const elements = $(selector);

    if (!elements || elements.length === 0) {
      continue;
    }

    found = true;

    elements.each((i, el) => {
      // ------------------------------
      // Brand
      // ------------------------------

      const brand = firstText(el, [
        ".product-brand",
        "[class*='product-brand']",
      ]);

      // ------------------------------
      // Product name
      // ------------------------------

      const name = firstText(el, [
        ".product-product",
        "[class*='product-product']",
        ".product-productName",
        "[class*='product-name']",
      ]);

      if (!brand && !name) {
        return;
      }

      // ------------------------------
      // Product URL
      // ------------------------------

      const externalUrl = absoluteUrl(
        $(el).find("a[href]").first().attr("href"),
      );

      // Use URL as a stronger dedup key.
      const productId =
        $(el).attr("id") || externalUrl || `${brand}-${name}-${i}`;

      if (seen.has(productId)) {
        return;
      }

      seen.add(productId);

      // ------------------------------
      // CURRENT PRICE
      //
      // Myntra commonly has:
      // .product-discountedPrice
      //
      // But we also check multiple fallbacks.
      // ------------------------------

      const currentPriceText = firstText(el, [
        ".product-discountedPrice",
        "[class*='discountedPrice']",
        "[class*='discounted-price']",
        ".product-price",
        "[class*='product-price']",
      ]);

      let currentPrice = parsePrice(currentPriceText);

      // Fallback: inspect the whole card for ₹ amount.
      if (currentPrice === null) {
        const cardText = cleanText($(el).text());

        const priceMatches = cardText.match(
          /(?:₹|Rs\.?|INR)\s*[\d,]+(?:\.\d{1,2})?/gi,
        );

        if (priceMatches && priceMatches.length > 0) {
          currentPrice = parsePrice(priceMatches[0]);
        }
      }

      // ------------------------------
      // ORIGINAL / MRP
      // ------------------------------

      const originalPriceText = firstText(el, [
        ".product-strike",
        "[class*='product-strike']",
        "[class*='strike']",
        "[class*='mrp']",
        "[class*='MRP']",
      ]);

      let originalPrice = parsePrice(originalPriceText);

      if (originalPrice === null) {
        originalPrice = currentPrice;
      }

      // Never allow an invalid price < ₹1.
      if (currentPrice !== null && currentPrice < 1) {
        currentPrice = null;
      }

      if (originalPrice !== null && originalPrice < 1) {
        originalPrice = currentPrice;
      }

      // ------------------------------
      // Rating
      // ------------------------------

      const rating = parseRating(
        firstText(el, [
          ".product-ratingsContainer span",
          "[class*='ratingsContainer'] span",
          "[class*='rating']",
        ]),
      );

      // ------------------------------
      // Image
      // ------------------------------

      const imageUrl = getImageUrl(el, ["img.img-responsive", "img"]);

      // ------------------------------
      // Product
      // ------------------------------

      items.push({
        retailerName: "Myntra",
        query: getInputQuery(),

        productId: productId,

        title: cleanText([brand, name].filter(Boolean).join(" - ")),

        brand: brand || null,

        currentPrice: currentPrice,
        originalPrice: originalPrice,

        currency: "INR",

        rating: rating,
        reviewCount: null,

        imageUrl: imageUrl,
        externalUrl: externalUrl,

        scrapedAt: new Date().toISOString(),
      });
    });

    // If one selector worked, don't process the
    // same cards again using the fallback selector.
    if (found) {
      break;
    }
  }

  return items;
}

// ============================================================
// AJIO
// ============================================================
//
// AJIO is the most fragile of the three because its product-card
// class names have changed over time.
//
// Therefore this parser intentionally uses several fallback
// selectors instead of depending only on:
//
//     .item
//     .brand
//     .nameCls
//
// ============================================================

function parseAjioResults() {
  const items = [];
  const seen = new Set();

  const cardSelectors = [
    ".item",
    ".items .item",
    "[class*='item']",
    "[class*='product-card']",
    "[class*='productCard']",
    "[class*='product-card-wrapper']",
    "[class*='productCardWrapper']",
  ];

  let cards = null;

  for (const selector of cardSelectors) {
    const found = $(selector);

    if (found && found.length > 0) {
      cards = found;
      break;
    }
  }

  // ----------------------------------------------------------
  // Last-resort strategy:
  //
  // Find links that appear to point to product pages and use
  // their nearest reasonable parent as the product card.
  // ----------------------------------------------------------

  if (!cards || cards.length === 0) {
    const possibleCards = [];

    $("a[href]").each((i, el) => {
      const href = $(el).attr("href") || "";

      if (!href.includes("/p/")) {
        return;
      }

      let parent = $(el).parent();

      for (let depth = 0; depth < 5; depth++) {
        if (!parent || parent.length === 0) {
          break;
        }

        const text = cleanText(parent.text());

        if (text.length >= 10) {
          possibleCards.push(parent);
          break;
        }

        parent = parent.parent();
      }
    });

    cards = possibleCards;
  }

  if (!cards || cards.length === 0) {
    return [];
  }

  cards.each((i, el) => {
    // --------------------------------------------------------
    // BRAND
    // --------------------------------------------------------

    const brand = firstText(el, [
      ".brand",
      ".brand-name",
      "[class*='brand']",
      "[class*='Brand']",
    ]);

    // --------------------------------------------------------
    // PRODUCT NAME
    // --------------------------------------------------------

    const name = firstText(el, [
      ".nameCls",
      ".name",
      ".product-name",
      "[class*='nameCls']",
      "[class*='product-name']",
      "[class*='productName']",
      "[class*='name']",
    ]);

    // --------------------------------------------------------
    // PRODUCT URL
    // --------------------------------------------------------

    const externalUrl = absoluteUrl($(el).find("a[href]").first().attr("href"));

    // AJIO product URLs normally contain /p/
    // but don't require it because some page structures
    // may use different URL patterns.
    const productId =
      $(el).attr("data-id") ||
      $(el).attr("data-product-id") ||
      $(el).attr("data-code") ||
      externalUrl ||
      `${brand}-${name}-${i}`;

    // --------------------------------------------------------
    // Skip useless containers
    // --------------------------------------------------------

    if (!brand && !name && !externalUrl) {
      return;
    }

    // --------------------------------------------------------
    // Deduplicate
    // --------------------------------------------------------

    if (seen.has(productId)) {
      return;
    }

    seen.add(productId);

    // --------------------------------------------------------
    // CURRENT PRICE
    // --------------------------------------------------------

    const currentPriceText = firstText(el, [
      ".price strong",
      ".price",
      ".current-price",
      ".selling-price",
      "[class*='selling-price']",
      "[class*='sellingPrice']",
      "[class*='current-price']",
      "[class*='currentPrice']",
      "[class*='price']",
    ]);

    let currentPrice = parsePrice(currentPriceText);

    // --------------------------------------------------------
    // Fallback: inspect card text for INR prices
    // --------------------------------------------------------

    if (currentPrice === null) {
      const cardText = cleanText($(el).text());

      const priceMatches = cardText.match(
        /(?:₹|Rs\.?|INR)\s*[\d,]+(?:\.\d{1,2})?/gi,
      );

      if (priceMatches && priceMatches.length > 0) {
        currentPrice = parsePrice(priceMatches[0]);
      }
    }

    // --------------------------------------------------------
    // ORIGINAL / MRP
    // --------------------------------------------------------

    const originalPriceText = firstText(el, [
      ".orginal-price",
      ".original-price",
      ".originalPrice",
      ".mrp",
      "[class*='orginal-price']",
      "[class*='original-price']",
      "[class*='originalPrice']",
      "[class*='mrp']",
      "[class*='MRP']",
    ]);

    let originalPrice = parsePrice(originalPriceText);

    if (originalPrice === null) {
      originalPrice = currentPrice;
    }

    // --------------------------------------------------------
    // Reject malformed decimal prices
    // --------------------------------------------------------

    if (currentPrice !== null && currentPrice < 1) {
      currentPrice = null;
    }

    if (originalPrice !== null && originalPrice < 1) {
      originalPrice = currentPrice;
    }

    // --------------------------------------------------------
    // RATING
    // --------------------------------------------------------

    const rating = parseRating(
      firstText(el, [
        ".rating-value",
        ".rating",
        "[class*='rating-value']",
        "[class*='ratingValue']",
        "[class*='rating']",
      ]),
    );

    // --------------------------------------------------------
    // IMAGE
    // --------------------------------------------------------

    const imageUrl = getImageUrl(el, ["img", "source"]);

    // --------------------------------------------------------
    // TITLE
    // --------------------------------------------------------

    const title = cleanText([brand, name].filter(Boolean).join(" - "));

    // If we couldn't get a brand/name, don't emit an empty
    // product.
    if (!title) {
      return;
    }

    // --------------------------------------------------------
    // OUTPUT
    // --------------------------------------------------------

    items.push({
      retailerName: "Ajio",
      query: getInputQuery(),

      productId: productId,

      title: title,
      brand: brand || null,

      currentPrice: currentPrice,
      originalPrice: originalPrice,

      currency: "INR",

      rating: rating,
      reviewCount: null,

      imageUrl: imageUrl,
      externalUrl: externalUrl,

      scrapedAt: new Date().toISOString(),
    });
  });

  return items;
}

// ============================================================
// MAIN PARSER
// ============================================================
//
// IMPORTANT:
// Keep parser synchronous.
// parse() from Interaction Code expects the actual array,
// not a Promise.
// ============================================================

function main() {
  const retailer = detectRetailer();

  if (retailer === "amazon") {
    return parseAmazonResults();
  }

  if (retailer === "myntra") {
    return parseMyntraResults();
  }

  if (retailer === "ajio") {
    return parseAjioResults();
  }

  return [];
}

return main();
