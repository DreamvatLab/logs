import { makeAutoObservable, observable, reaction, runInAction } from "mobx";
import { getListData, getTableData } from "@/services";
import dayjs, { Dayjs } from "dayjs";
import { LogEntry, TableDataSource } from "@/models";

class HomeVM {
    pageIndex: number = 1;
    pageSize: number = 25;

    clients: string[] = [];
    client?: string;

    databases: string[] = [];
    database?: string;

    tables: string[] = [];
    table?: string;

    levels = [
        { value: -1, label: "All", },
        { value: 0, label: "Verbose", },
        { value: 1, label: "Debug", },
        { value: 2, label: "Info", },
        { value: 3, label: "Warning", },
        { value: 4, label: "Error", },
        { value: 5, label: "Fatal", },
    ];
    level: number = -1;

    flags?: number;
    // fuzzySearch: boolean = false;
    message?: string;
    user?: string;
    traceNo?: string;
    fromTime?: string;
    toTime?: string;
    get dateRange(): any {
        if (this.fromTime && this.toTime) {
            return [dayjs(this.fromTime), dayjs(this.toTime)];
        } else if (this.fromTime) {
            return [dayjs(this.fromTime), null];
        } else if (this.toTime) {
            return [null, dayjs(this.toTime)];
        }

        return [null, null]
    }

    loading: boolean = false;
    total: number = 0;
    data?: LogEntry[];

    constructor() {
        this.fromTime = dayjs().subtract(1, 'month').format();

        makeAutoObservable(this, {
            // flags: false,
            // fuzzySearch: false,
            // message: false,
            // user: false,
            // traceNo: false,
            // fromTime: false,
            // toTime: false,
        });

        // Event when selected Client changes
        reaction(() => this.client, async () => {
            // console.log(this.client);
            const rs = await getListData(this.client ?? "", this.database ?? "");
            runInAction(() => {
                this.databases = rs.Databases;
                // Select the first one
                if (this.databases.length) {
                    this.database = this.databases[0];
                }
            });
        });
        // Event when selected database changes
        reaction(() => this.database, async () => {
            const rs = await getListData(this.client ?? "", this.database ?? "");
            console.log(rs);
            runInAction(() => {
                this.tables = rs.Tables;
                // Select the first one
                if (this.tables.length) {
                    this.table = this.tables[0];
                }
            });
        });
        // Event when tables change
        reaction(() => this.table, () => {
            if (!this.table) {
                return;
            }

            const currentYear = dayjs().year();
            const selectedYear = parseInt(this.table);

            if (isNaN(selectedYear)) {
                return;
            }

            if (selectedYear !== currentYear) {
                this.fromTime = dayjs().year(selectedYear).startOf('year').format();
            } else {
                // For current year, default to 30 days ago
                this.fromTime = dayjs().subtract(1, 'month').format();
            }
        });
    }

    async init() {
        // Read dropdown list information
        const rs = await getListData(this.client ?? "", this.database ?? "");

        runInAction(() => {
            this.clients = rs.Clients;
        });
    }

    async search() {
        if (!this.database || !this.table) {
            return;
        }
        this.loading = true;

        const params = {
            PageSize: this.pageSize,
            PageIndex: this.pageIndex,
            DBName: this.database!,
            TableName: this.table!,
            Message: this.message,
            User: this.user,
            TraceNo: this.traceNo,
            StartTime: this.fromTime,
            EndTime: this.toTime,
            Level: this.level,
            Flags: this.flags,
        };

        // if (this.fuzzySearch) {
        //     params.Flags ??= 0;
        //     params.Flags |= 1;
        // }

        const rs = await getTableData(params);

        runInAction(() => {
            this.data = rs.list;
            this.total = rs.total;
            this.loading = false;
        });
    }
    async setPageIndex(v: number) {
        this.pageIndex = v;

        await this.search();
    }

    async reset() {
        this.message = undefined;
        this.user = undefined;
        this.traceNo = undefined;
        this.flags = undefined;
        this.fromTime = dayjs().subtract(1, 'month').format();
        this.toTime = undefined;
        // this.fuzzySearch = false;
        this.level = -1;

        await this.search();
    }

    setClient(v: string) {
        this.client = v;
    }
    setDatabase(v: string) {
        this.database = v;
    }
    setTable(v: string) {
        this.table = v;
    }
    setLevel(v: number) {
        this.level = v;
    }
    setMessage(v: string) {
        this.message = v;
    }
    setUser(v: string) {
        this.user = v;
    }
    setTraceNo(v: string) {
        this.traceNo = v;
    }
    setFlags(v?: number) {
        this.flags = v;
    }
    // setFuzzySearch(v: boolean) {
    //     this.fuzzySearch = v;
    // }
    setDateRange(v: any) {
        const [from, to] = v;

        this.fromTime = from?.format();
        this.toTime = to?.format();
    }
}

export default HomeVM;