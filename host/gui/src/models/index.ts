export interface ListData {
    Client: string,
    Database: string,
    Table: string,
    Clients: string[],
    Databases: string[],
    Tables: string[],
}

export interface LogEntry {
    ID: string,
    TraceNo: string,
    User: string,
    Message: string,
    Error: string,
    StackTrace: string,
    Level: number,
    CreatedOnUtc: number,
}

export interface SearchParams {
    PageSize: number;
    PageIndex: number;
    DBName: string;
    TableName: string;
    User?: string;
    TraceNo?: string;
    Message?: string;
    StartTime?: string;
    EndTime?: string;
    Level?: number;
    Flags?: number;
}

export interface TableDataSource {
    total: number;
    list: LogEntry[];
}