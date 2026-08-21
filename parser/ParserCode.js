function detectRetailer() {
  const href = location.href;
  if (href.includes("amazon.in")) return "amazon";
  if (href.includes("myntra.com")) return "myntra";
  if (href.includes("ajio.com")) return "ajio";
  return null;
}

function getInputQuery() {
  const query =
    input && typeof input.query === "string" ? input.query.trim() : "";
  return query || null;
}

function parsePrice(text) {
  if (!text) return null;
  const cleaned = text.replace(/[^\d.]/g, "");
  return cleaned ? parseFloat(cleaned) : null;
}

function absoluteUrl(url) {
  if (!url) return null;
  try {
    return new URL(url, location.href).href;
  } catch {
    return url;
  }
}

// ---------- Amazon search results ----------
function parseAmazonResults() {
  const items = [];
  $(".s-main-slot .s-result-item[data-asin]").each((i, el) => {
    const asin = $(el).attr("data-asin");
    if (!asin) return;

    const title = $(el).find("h2 span").text_sane();
    if (!title) return;

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
    const currentPrice = priceWhole
      ? parseFloat(
          priceFraction ? `${priceWhole}.${priceFraction}` : priceWhole,
        )
      : null;

    const originalPriceText = $(el)
      .find(".a-price.a-text-price .a-offscreen")
      .first()
      .text();
    const originalPrice = parsePrice(originalPriceText) || null;

    items.push({
      retailerName: "Amazon",
      query: getInputQuery(),
      productId: asin,
      title: title,
      brand: null,
      currentPrice: currentPrice,
      originalPrice: originalPrice,
      currency: "INR",
      rating: parsePrice(
        $(el).find(".a-icon-star-small .a-icon-alt").first().text_sane(),
      ),
      reviewCount:
        parseInt(
          (
            $(el).find(".a-size-base.s-underline-text").first().text() || ""
          ).replace(/\D/g, ""),
        ) || null,
      imageUrl: $(el).find("img.s-image").attr("src") || null,
      externalUrl: absoluteUrl($(el).find("h2 a").attr("href")),
      scrapedAt: new Date().toISOString(),
    });
  });
  return items;
}

// ---------- Myntra search results ----------
function parseMyntraResults() {
  const items = [];
  $(".product-base").each((i, el) => {
    const brand = $(el).find(".product-brand").text_sane();
    const name = $(el).find(".product-product").text_sane();
    if (!brand && !name) return;

    const currentPrice = parsePrice(
      $(el).find(".product-discountedPrice").text(),
    );
    const originalPrice =
      parsePrice($(el).find(".product-strike").text()) || currentPrice;

    items.push({
      retailerName: "Myntra",
      query: getInputQuery(),
      productId: $(el).attr("id") || null,
      title: [brand, name].filter(Boolean).join(" - "),
      brand: brand || null,
      currentPrice: currentPrice,
      originalPrice: originalPrice,
      currency: "INR",
      rating: parsePrice(
        $(el).find(".product-ratingsContainer span").first().text(),
      ),
      reviewCount: null,
      imageUrl:
        $(el).find("img.img-responsive").attr("src") ||
        $(el).find("img.img-responsive").attr("data-src") ||
        null,
      externalUrl: absoluteUrl($(el).find("a").first().attr("href")),
      scrapedAt: new Date().toISOString(),
    });
  });
  return items;
}

// ---------- Ajio search results ----------
function parseAjioResults() {
  const items = [];
  $(".item").each((i, el) => {
    const brand = $(el).find(".brand").text_sane();
    const name = $(el).find(".nameCls").text_sane();
    if (!brand && !name) return;

    const currentPrice = parsePrice($(el).find(".price strong").first().text());
    const originalPrice =
      parsePrice($(el).find(".orginal-price").first().text()) || currentPrice;

    items.push({
      retailerName: "Ajio",
      query: getInputQuery(),
      productId: $(el).attr("data-id") || null,
      title: [brand, name].filter(Boolean).join(" - "),
      brand: brand || null,
      currentPrice: currentPrice,
      originalPrice: originalPrice,
      currency: "INR",
      rating: parsePrice($(el).find(".rating-value").first().text()),
      reviewCount: null,
      imageUrl:
        $(el).find("img").attr("data-src") ||
        $(el).find("img").attr("src") ||
        null,
      externalUrl: absoluteUrl($(el).find("a").first().attr("href")),
      scrapedAt: new Date().toISOString(),
    });
  });
  return items;
}

function main() {
  const retailer = detectRetailer();
  if (retailer === "amazon") return parseAmazonResults();
  if (retailer === "myntra") return parseMyntraResults();
  if (retailer === "ajio") return parseAjioResults();
  return [];
}

return main();
