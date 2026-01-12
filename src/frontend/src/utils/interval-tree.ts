class IntervalNode<T> {
    start: number | bigint
    end: number | bigint
    max: number | bigint
    data: T
    left: IntervalNode<T> | null = null
    right: IntervalNode<T> | null = null

    constructor(start: number | bigint, end: number | bigint, data: T) {
        this.start = start
        this.end = end
        this.max = end
        this.data = data
    }
}

export class IntervalTree<T> {
    private intervals: Array<{
        start: number | bigint
        end: number | bigint
        data: T
    }> = []
    private root: IntervalNode<T> | null = null
    private isBuilt = false

    insert(start: number | bigint, end: number | bigint, data: T): void {
        this.intervals.push({ start, end, data })
        this.isBuilt = false
    }

    build(): void {
        if (this.intervals.length === 0) {
            this.root = null
            this.isBuilt = true
            return
        }

        this.intervals.sort((a, b) => {
            const diff = Number(a.start) - Number(b.start)
            return diff
        })

        this.root = this.buildTree(0, this.intervals.length - 1)
        this.isBuilt = true
    }

    private buildTree(left: number, right: number): IntervalNode<T> | null {
        if (left > right) {
            return null
        }

        const mid = Math.floor((left + right) / 2)
        const interval = this.intervals[mid]
        const node = new IntervalNode(interval.start, interval.end, interval.data)

        node.left = this.buildTree(left, mid - 1)
        node.right = this.buildTree(mid + 1, right)

        node.max = interval.end
        if (node.left && this.compare(node.left.max, node.max) > 0) {
            node.max = node.left.max
        }
        if (node.right && this.compare(node.right.max, node.max) > 0) {
            node.max = node.right.max
        }

        return node
    }

    query(start: number | bigint, end: number | bigint): T[] {
        if (!this.isBuilt) {
            throw new Error(`IntervalTree: build() must be called before query()`)
        }

        const results: T[] = []
        this.queryNode(this.root, start, end, results)
        return results
    }

    private queryNode(
        node: IntervalNode<T> | null,
        start: number | bigint,
        end: number | bigint,
        results: T[]
    ): void {
        if (!node) {
            return
        }

        if (this.compare(node.start, end) <= 0 && this.compare(start, node.end) <= 0) {
            results.push(node.data)
        }

        if (node.left && this.compare(node.left.max, start) >= 0) {
            this.queryNode(node.left, start, end, results)
        }
        if (node.right && this.compare(node.start, end) <= 0) {
            this.queryNode(node.right, start, end, results)
        }
    }

    private compare(a: number | bigint, b: number | bigint): number {
        if (typeof a === 'bigint' && typeof b === 'bigint') {
            if (a < b) return -1
            if (a > b) return 1
            return 0
        }
        return Number(a) - Number(b)
    }

    clear(): void {
        this.intervals = []
        this.root = null
        this.isBuilt = false
    }
}
