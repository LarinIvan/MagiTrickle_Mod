import { untrack } from 'svelte'
import type { Group, Rule } from '../../types'
import { IPUtils } from '../../utils/ip'
import { IntervalTree } from '../../utils/interval-tree'

export interface Conflict {
    id: string
    sourceRule: Rule
    sourceGroupId: string
    targetRule: Rule
    targetGroupId: string
    targetGroupName: string
    type: `exact` | `overlap`
}

interface RuleWithMeta {
    rule: Rule
    groupId: string
    groupName: string
}

class ConflictsManager {
    groupByRule = $state<Record<string, Conflict[]>>({})
    groupByGroup = $state<Record<string, Conflict[]>>({})
    activePopoverId = $state<string | null>(null)

    /**
     * Расчет всех конфликтов для списка групп
     * @param groups - массив групп для проверки
     */
    calculate(groups: Group[]): void {
        IPUtils.clearCache()

        this.groupByRule = {}
        this.groupByGroup = {}

        const allRules: RuleWithMeta[] = []

        for (const group of groups) {
            if (!group.enable) {
                continue
            }

            for (const rule of group.rules) {
                if (rule.enable) {
                    allRules.push
                        (
                            {
                                rule,
                                groupId: group.id,
                                groupName: group.name
                            }
                        )
                }
            }
        }

        this.detectExactDuplicates(allRules)
        this.detectIPOverlaps(allRules)
        this.detectHierarchicalConflicts(allRules)
    }

    private detectExactDuplicates(allRules: RuleWithMeta[]): void {
        const valueMap = new Map<string, RuleWithMeta[]>()

        for (const ruleWithMeta of allRules) {
            const val = ruleWithMeta.rule.rule.trim()

            if (!valueMap.has(val)) {
                valueMap.set(val, [])
            }

            valueMap.get(val)!.push(ruleWithMeta)
        }

        for (const [_, entries] of valueMap) {
            if (entries.length > 1) {
                for (let i = 0; i < entries.length; i++) {
                    for (let j = i + 1; j < entries.length; j++) {
                        const source = entries[i]
                        const target = entries[j]

                        this.addConflict
                            (
                                {
                                    id: `${source.rule.id}-${target.rule.id}`,
                                    sourceRule: source.rule,
                                    sourceGroupId: source.groupId,
                                    targetRule: target.rule,
                                    targetGroupId: target.groupId,
                                    targetGroupName: target.groupName,
                                    type: `exact`
                                }
                            )

                        this.addConflict
                            (
                                {
                                    id: `${target.rule.id}-${source.rule.id}`,
                                    sourceRule: target.rule,
                                    sourceGroupId: target.groupId,
                                    targetRule: source.rule,
                                    targetGroupId: source.groupId,
                                    targetGroupName: source.groupName,
                                    type: `exact`
                                }
                            )
                    }
                }
            }
        }
    }

    private detectHierarchicalConflicts(allRules: RuleWithMeta[]): void {
        interface ReversedRule extends RuleWithMeta {
            reversed: string
        }

        const candidates: ReversedRule[] = []

        for (const r of allRules) {
            if (r.rule.type === 'domain' || r.rule.type === 'namespace') {
                candidates.push({
                    ...r,
                    reversed: r.rule.rule.split('.').reverse().join('.')
                })
            }
        }

        if (candidates.length < 2) return

        candidates.sort((a, b) => a.reversed.localeCompare(b.reversed))

        const stack: ReversedRule[] = []

        for (const current of candidates) {
            while (stack.length > 0) {
                const parent = stack[stack.length - 1]

                if (current.reversed.startsWith(parent.reversed + '.') || current.reversed === parent.reversed) {
                    break
                } else {
                    stack.pop()
                }
            }

            for (const parent of stack) {
                if (parent.rule.rule === current.rule.rule) continue

                this.addConflict({
                    id: `${parent.rule.id}-${current.rule.id}`,
                    sourceRule: parent.rule,
                    sourceGroupId: parent.groupId,
                    targetRule: current.rule,
                    targetGroupId: current.groupId,
                    targetGroupName: current.groupName,
                    type: 'overlap'
                })

                this.addConflict({
                    id: `${current.rule.id}-${parent.rule.id}`,
                    sourceRule: current.rule,
                    sourceGroupId: current.groupId,
                    targetRule: parent.rule,
                    targetGroupId: parent.groupId,
                    targetGroupName: parent.groupName,
                    type: 'overlap'
                })
            }

            if (current.rule.type === 'namespace') {
                stack.push(current)
            }
        }
    }

    private detectIPOverlaps(allRules: RuleWithMeta[]): void {
        const ipv4Rules: RuleWithMeta[] = []
        const ipv6Rules: RuleWithMeta[] = []

        for (const ruleWithMeta of allRules) {
            const ruleType = ruleWithMeta.rule.type

            if (ruleType === `subnet`) {
                ipv4Rules.push(ruleWithMeta)
            }
            else if (ruleType === `subnet6`) {
                ipv6Rules.push(ruleWithMeta)
            }
        }

        if (ipv4Rules.length > 1) {
            this.detectIPOverlapsForType(ipv4Rules, false)
        }
        if (ipv6Rules.length > 1) {
            this.detectIPOverlapsForType(ipv6Rules, true)
        }
    }

    private detectIPOverlapsForType(rules: RuleWithMeta[], isIPv6: boolean): void {
        const tree = new IntervalTree<RuleWithMeta>()

        for (const ruleWithMeta of rules) {
            const parsed = IPUtils.parseCIDR(ruleWithMeta.rule.rule)

            if (!parsed || parsed.isIPv6 !== isIPv6) {
                continue
            }

            const range = isIPv6
                ? IPUtils.getIPv6Range(parsed)
                : IPUtils.getIPv4Range(parsed)

            if (range) {
                tree.insert(range.start, range.end, ruleWithMeta)
            }
        }

        tree.build()

        for (const ruleWithMeta of rules) {
            const parsed = IPUtils.parseCIDR(ruleWithMeta.rule.rule)

            if (!parsed || parsed.isIPv6 !== isIPv6) {
                continue
            }

            const range = isIPv6
                ? IPUtils.getIPv6Range(parsed)
                : IPUtils.getIPv4Range(parsed)

            if (!range) {
                continue
            }

            const overlaps = tree.query(range.start, range.end)

            for (const other of overlaps) {
                if (ruleWithMeta.rule.id === other.rule.id) {
                    continue
                }
                if (ruleWithMeta.rule.rule === other.rule.rule) {
                    continue
                }

                this.addConflict
                    (
                        {
                            id: `${ruleWithMeta.rule.id}-${other.rule.id}`,
                            sourceRule: ruleWithMeta.rule,
                            sourceGroupId: ruleWithMeta.groupId,
                            targetRule: other.rule,
                            targetGroupId: other.groupId,
                            targetGroupName: other.groupName,
                            type: `overlap`
                        }
                    )
            }
        }
    }

    private addConflict(conflict: Conflict): void {
        if (!this.groupByRule[conflict.sourceRule.id]) {
            this.groupByRule[conflict.sourceRule.id] = []
        }
        this.groupByRule[conflict.sourceRule.id].push(conflict)

        if (!this.groupByGroup[conflict.sourceGroupId]) {
            this.groupByGroup[conflict.sourceGroupId] = []
        }
        this.groupByGroup[conflict.sourceGroupId].push(conflict)
    }

    getConflictsForRule(ruleId: string): Conflict[] {
        return this.groupByRule[ruleId] || []
    }

    getConflictsForGroup(groupId: string): Conflict[] {
        const conflicts = this.groupByGroup[groupId] || []

        const seenIntraGroup = new Set<string>()
        const seenInterGroup = new Set<string>()
        const unique: Conflict[] = []

        for (const conflict of conflicts) {
            const isIntraGroup = conflict.sourceGroupId === conflict.targetGroupId

            if (isIntraGroup) {
                const key = [conflict.sourceRule.id, conflict.targetRule.id].sort().join(`-`)

                if (!seenIntraGroup.has(key)) {
                    seenIntraGroup.add(key)
                    unique.push(conflict)
                }
            }
            else {
                const key = `${conflict.targetGroupId}:${conflict.targetRule.rule}`

                if (!seenInterGroup.has(key)) {
                    seenInterGroup.add(key)
                    unique.push(conflict)
                }
            }
        }

        return unique
    }

    get hasAnyConflicts(): boolean {
        return Object.keys(this.groupByRule).length > 0
    }

    getAllConflicts(): Conflict[] {
        const seen = new Set<string>()
        const result: Conflict[] = []

        for (const ruleId in this.groupByRule) {
            for (const conflict of this.groupByRule[ruleId]) {
                const key = [conflict.sourceRule.id, conflict.targetRule.id].sort().join(`-`)
                if (!seen.has(key)) {
                    seen.add(key)
                    result.push(conflict)
                }
            }
        }

        return result
    }

    getConflictClusters(): { rules: Array<{ rule: Rule; groupId: string; groupName: string }> }[] {
        const conflicts = this.getAllConflicts()
        if (conflicts.length === 0) return []

        const ruleMap = new Map<string, { rule: Rule; groupId: string; groupName: string }>()

        for (const c of conflicts) {
            if (!ruleMap.has(c.sourceRule.id)) {
                ruleMap.set(c.sourceRule.id, {
                    rule: c.sourceRule,
                    groupId: c.sourceGroupId,
                    groupName: ``
                })
            }
            if (!ruleMap.has(c.targetRule.id)) {
                ruleMap.set(c.targetRule.id, {
                    rule: c.targetRule,
                    groupId: c.targetGroupId,
                    groupName: c.targetGroupName
                })
            }
        }

        const parent = new Map<string, string>()

        const find = (x: string): string => {
            if (!parent.has(x)) parent.set(x, x)
            if (parent.get(x) !== x) parent.set(x, find(parent.get(x)!))
            return parent.get(x)!
        }

        const union = (a: string, b: string) => {
            const ra = find(a)
            const rb = find(b)
            if (ra !== rb) parent.set(ra, rb)
        }

        for (const c of conflicts) {
            union(c.sourceRule.id, c.targetRule.id)
        }
        const clusters = new Map<string, Array<{ rule: Rule; groupId: string; groupName: string }>>()

        for (const [ruleId, info] of ruleMap) {
            const root = find(ruleId)
            if (!clusters.has(root)) clusters.set(root, [])
            clusters.get(root)!.push(info)
        }

        return Array.from(clusters.values()).map(rules => ({ rules }))
    }
}

export const conflictsStore = new ConflictsManager()
