-- Default Lua Script (GoQuery Enabled)
-- Globals: fetch(url), html_parse(html), ARG_FULL_URL, ARG_PAGE_TYPE

function parse_list(url)
    local body, err = fetch(url)
    if err then
        return {type = "error", error = err}
    end

    local doc = html_parse(body)
    local results = {}
    
    -- Using CSS Selectors (similar to Cheerio)
    doc:find("table tr"):each(function(i, s)
        -- Skip headers
        if s:find("th"):count() > 0 then return end
        
        -- Extract name and link (from column 1)
        local a = s:find("td"):eq(0):find("a"):last()
        local name = a:text()
        local relative_url = a:attr("href")
        
        if name ~= "" and relative_url ~= "" then
            -- Resolve URL
            local full_item_url = relative_url
            if string.sub(relative_url, 1, 4) ~= "http" then
                local domain = string.match(url, "(https?://[^/]+)")
                full_item_url = domain .. relative_url
            end

            -- Seeds, Leeches, Size (Adjust selectors based on site)
            local seeds = tonumber(s:find("td"):eq(1):text()) or 0
            local leeches = tonumber(s:find("td"):eq(2):text()) or 0
            local size = s:find("td"):eq(4):text()
            if size == "" then size = "Unknown" end

            table.insert(results, {
                name = name,
                url = full_item_url,
                seeds = seeds,
                leeches = leeches,
                size = size
            })
        end
    end)

    return {type = "list", results = results}
end

function parse_detail(url)
    local body, err = fetch(url)
    if err then
        return {type = "error", error = err}
    end
    
    local doc = html_parse(body)
    
    -- Extract Name from h1 or title
    local name = doc:find("h1"):first():text()
    if name == "" then name = doc:find("title"):text() end

    -- Find magnet link
    local magnet = doc:find("a[href^='magnet:']"):attr("href")
    
    -- Find direct downloads
    local downloads = {}
    doc:find("a"):each(function(i, s)
        local href = s:attr("href")
        if string.find(href, ".torrent") or string.find(href, "download") then
            if not string.find(href, "magnet:") then
                table.insert(downloads, {
                    url = href,
                    text = s:text() or "Download"
                })
            end
        end
    end)
    
    return {
        type = "detail",
        name = name,
        magnetLink = magnet,
        directDownloads = downloads
    }
end

-- Main execution
if ARG_PAGE_TYPE == "list" then
    return parse_list(ARG_FULL_URL)
elseif ARG_PAGE_TYPE == "detail" then
    return parse_detail(ARG_FULL_URL)
else
    return parse_list(ARG_FULL_URL)
end
