import { LOCAL_API_ROOT } from "@/constants";
import { ListData, SearchParams, TableDataSource } from "@/models";
// import moment, { Moment } from "moment";

export const getTableData = async (params: SearchParams): Promise<TableDataSource> => {
    var url = LOCAL_API_ROOT + "/logs"
    // console.log(formData);

    // var body = {
    //     PageSize: params.PageSize,
    //     PageIndex: params.PageIndex,
    //     DBName: params.DBName,
    //     TableName: params.TableName,
    //     User: params.User?.trim(),
    //     TraceNo: params.TraceNo?.trim(),
    //     Message: params.Message?.trim(),
    //     StartTime: "",
    //     EndTime: "",
    //     Level: params.Level,
    //     Flags: 0,
    // };

    // // date range
    // var dateRange: Dayjs[] = formData.DateRange;
    // if (dateRange?.length == 2) {
    //     var start = dateRange[0];
    //     var end = dateRange[1];
    //     if (start) {
    //         var startTime = dayjs(start);  // Must clone
    //         // body.StartTime = startTime.startOf('day').utc().format();
    //     }
    //     if (end) {
    //         var endTime = dayjs(end);      // Must clone
    //         // body.EndTime = endTime.startOf('day').utc().format();
    //     }
    // }
    // // flags, fuzzy search
    // if (formData.FuzzySearch) {
    //     body.Flags |= 1;
    // }

    // console.log(body);

    const resp = await fetch(url, {
        method: "POST",
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify(params),
    });
    const jsonStr = await resp.text();
    
    const r = JSON.parse(jsonStr);

    return {
        list: r.LogEntries,
        total: r.TotalCount,
    };
};

export const getListData = async (client: string, db: string): Promise<ListData> => {
    var url = LOCAL_API_ROOT + `/listData?client=${client}&db=${db}`

    const resp = await fetch(url);
    const jsonStr = await resp.text();
    const r: ListData = JSON.parse(jsonStr);
    return r;
};