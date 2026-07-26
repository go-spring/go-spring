-- guard.lua — an HTTP request filter running at the net/http layer, in front
-- of whatever web framework serves the routes. It demonstrates the three
-- things a data-plane Lua filter typically does: observe, mutate, and gate.

-- observe: log every incoming request through the go-spring log pipeline.
log("incoming " .. req.method .. " " .. req.path)

-- mutate: tag every response so downstream/clients can see the filter ran.
resp.set_header("X-Lua-Filter", "guard")

-- gate: block /admin unless the request carries the expected token. deny()
-- writes the response and short-circuits the chain, so the business handler
-- is never reached. Always return right after deny().
if req.path == "/admin" then
    if req.header("X-Token") ~= "sesame" then
        deny(403, "forbidden: bad token")
        return
    end
end
