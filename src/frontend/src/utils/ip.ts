import * as ipaddr from 'ipaddr.js'

export interface ParsedCIDR {
    address: ipaddr.IPv4 | ipaddr.IPv6
    prefix: number
    isIPv6: boolean
}

const cidrCache = new Map<string, ParsedCIDR | null>()

export class IPUtils {
    static parseCIDR(cidr: string): ParsedCIDR | null {
        if (cidrCache.has(cidr)) {
            return cidrCache.get(cidr)!
        }

        try {
            const parts = cidr.split(`/`)
            if (parts.length !== 2) {
                cidrCache.set(cidr, null)
                return null
            }

            const addr = ipaddr.process(parts[0])
            const prefix = parseInt(parts[1], 10)

            if (isNaN(prefix)) {
                cidrCache.set(cidr, null)
                return null
            }

            const result: ParsedCIDR =
            {
                address: addr,
                prefix: prefix,
                isIPv6: addr.kind() === `ipv6`
            }

            cidrCache.set(cidr, result)
            return result
        }
        catch (e) {
            cidrCache.set(cidr, null)
            return null
        }
    }

    static ipv4ToNumber(ipv4: ipaddr.IPv4): number {
        const parts = ipv4.octets
        return (parts[0] << 24) | (parts[1] << 16) | (parts[2] << 8) | parts[3]
    }

    static ipv6ToBigInt(ipv6: ipaddr.IPv6): bigint {
        const parts = ipv6.parts
        let result = BigInt(0)
        for (let i = 0; i < parts.length; i++) {
            result = (result << BigInt(16)) | BigInt(parts[i])
        }
        return result
    }

    static getIPv4Range(parsed: ParsedCIDR): { start: number; end: number } | null {
        if (parsed.isIPv6 || parsed.prefix < 0 || parsed.prefix > 32) {
            return null
        }

        const ipv4 = parsed.address as ipaddr.IPv4
        const ip = this.ipv4ToNumber(ipv4)

        const mask = parsed.prefix === 0 ? 0 : (0xFFFFFFFF << (32 - parsed.prefix)) >>> 0
        const start = (ip & mask) >>> 0
        const end = (start | (~mask & 0xFFFFFFFF)) >>> 0

        return { start, end }
    }

    static getIPv6Range(parsed: ParsedCIDR): { start: bigint; end: bigint } | null {
        if (!parsed.isIPv6 || parsed.prefix < 0 || parsed.prefix > 128) {
            return null
        }

        const ipv6 = parsed.address as ipaddr.IPv6
        const ip = this.ipv6ToBigInt(ipv6)

        const mask = parsed.prefix === 0 ? BigInt(0) : ((BigInt(1) << BigInt(128 - parsed.prefix)) - BigInt(1))
        const invertedMask = ~mask & ((BigInt(1) << BigInt(128)) - BigInt(1))
        const start = ip & invertedMask
        const end = start | mask

        return { start, end }
    }

    static isOverlap(cidrA: string, cidrB: string): boolean {
        const parsedA = this.parseCIDR(cidrA)
        const parsedB = this.parseCIDR(cidrB)

        if (!parsedA || !parsedB) {
            return false
        }

        if (parsedA.isIPv6 !== parsedB.isIPv6) {
            return false
        }

        if (parsedA.isIPv6) {
            const rangeA = this.getIPv6Range(parsedA)
            const rangeB = this.getIPv6Range(parsedB)

            if (!rangeA || !rangeB) {
                return false
            }

            return rangeA.start <= rangeB.end && rangeB.start <= rangeA.end
        }
        else {
            const rangeA = this.getIPv4Range(parsedA)
            const rangeB = this.getIPv4Range(parsedB)

            if (!rangeA || !rangeB) {
                return false
            }

            return rangeA.start <= rangeB.end && rangeB.start <= rangeA.end
        }
    }

    static clearCache(): void {
        cidrCache.clear()
    }
}
